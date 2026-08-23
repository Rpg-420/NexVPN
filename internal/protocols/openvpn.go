package protocols

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/sysproc"
)

type OpenVPNAdapter struct {
	mu        sync.RWMutex
	cmd       *exec.Cmd
	running   bool
	bytesRx   int64
	bytesTx   int64
	tmpConfig string
	tmpCreds  string
	username  string
	password  string
	logger    LoggerFunc
	onExit    OnExitFunc
	lockFile  string
}

func NewOpenVPNAdapter() *OpenVPNAdapter {
	return &OpenVPNAdapter{}
}

func (a *OpenVPNAdapter) Name() config.Protocol {
	return config.ProtocolOpenVPN
}

func (a *OpenVPNAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	a.mu.Unlock()
}

// SetOnExit registers a callback invoked when the OpenVPN process exits unexpectedly.
func (a *OpenVPNAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.Lock()
	a.onExit = fn
	a.mu.Unlock()
}

func (a *OpenVPNAdapter) SetCredentials(username, password string) {
	a.mu.Lock()
	a.username = username
	a.password = password
	a.mu.Unlock()
}

func (a *OpenVPNAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "openvpn")
	}
}

func sanitizeOpenVPNConfig(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var cleaned []string

	// Detect if config uses TCP
	isTCP := false
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "proto tcp") || strings.Contains(lowerContent, "proto tcp4") || strings.Contains(lowerContent, "proto tcp6") {
		isTCP = true
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		lowerLine := strings.ToLower(trimmed)

		// If TCP mode, remove or comment out explicit-exit-notify
		if isTCP && strings.HasPrefix(lowerLine, "explicit-exit-notify") {
			continue
		}

		// Fix unclosed quotes on individual lines
		quoteCount := strings.Count(trimmed, "\"")
		if quoteCount%2 != 0 {
			if strings.HasSuffix(trimmed, "\"") {
				trimmed = strings.TrimSuffix(trimmed, "\"")
			} else {
				trimmed = trimmed + "\""
			}
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "\n")
}

func (a *OpenVPNAdapter) Start(ctx context.Context, configContent string) error {
	ovpnBin, err := findBinary("openvpn")
	if err != nil {
		return fmt.Errorf("OpenVPN binary not found. Please install OpenVPN or place openvpn binary in bin folder")
	}

	sanitized := sanitizeOpenVPNConfig(configContent)

	a.mu.RLock()
	username := a.username
	password := a.password
	a.mu.RUnlock()

	var credsPath string
	if username != "" && password != "" {
		// Fix #15: Use dedicated secure temp dir for credentials
		credsFile, err := os.CreateTemp(ensureNexVPNTempDir(), "openvpn-creds-*.txt")
		if err == nil {
			credsPath = credsFile.Name()
			_, _ = credsFile.WriteString(fmt.Sprintf("%s\n%s\n", username, password))
			credsFile.Close()
		}
	}

	// Fix #15: Use dedicated secure temp dir
	tmpDir := ensureNexVPNTempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "openvpn-*.ovpn")
	if err != nil {
		if credsPath != "" {
			_ = os.Remove(credsPath)
		}
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(sanitized); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		if credsPath != "" {
			_ = os.Remove(credsPath)
		}
		return err
	}
	tmpFile.Close()

	binDir := filepath.Dir(ovpnBin)

	// Ensure wintun.dll is available in binDir
	wintunPath := filepath.Join(binDir, "wintun.dll")
	if _, statErr := os.Stat(wintunPath); os.IsNotExist(statErr) {
		if srcWintun, findErr := findBinary("wintun.dll"); findErr == nil {
			if data, readErr := os.ReadFile(srcWintun); readErr == nil {
				_ = os.WriteFile(wintunPath, data, 0755)
			}
		}
	}

	args := []string{
		"--config", tmpPath,
		"--verb", "3",
		"--suppress-timestamps",
		"--replay-window", "512", "60",
		"--mute-replay-warnings",
		"--ping", "10",
		"--ping-restart", "60",
		"--mssfix", "1360",
		"--auth-nocache",
		"--route-delay", "2",
	}

	statusFile, err := os.CreateTemp(ensureNexVPNTempDir(), "openvpn-status-*.log")
	var statusFilePath string
	if err == nil {
		statusFilePath = statusFile.Name()
		statusFile.Close()
		args = append(args, "--status", statusFilePath, "1", "--status-version", "3")
	}

	if credsPath != "" {
		args = append(args, "--auth-user-pass", credsPath)
	}

	var cmd *exec.Cmd
	var lockFilePath string

	u, errUser := user.Current()
	isRoot := errUser == nil && u.Uid == "0"

	if runtime.GOOS == "linux" && !isRoot && !hasNetCap(ovpnBin) {
		if _, err := exec.LookPath("pkexec"); err == nil {
			lockFile, err := os.CreateTemp(ensureNexVPNTempDir(), "openvpn-lock-*.lock")
			if err == nil {
				lockFilePath = lockFile.Name()
				lockFile.Close()
				script := `
					lock="$1"
					shift
					"$@" &
					pid=$!
					while kill -0 $pid 2>/dev/null && [ -f "$lock" ]; do
						sleep 0.5
					done
					kill -9 $pid 2>/dev/null
					exit 0
				`
				pkArgs := append([]string{"sh", "-c", script, "sh", lockFilePath, ovpnBin}, args...)
				cmd = exec.CommandContext(ctx, "pkexec", pkArgs...)
			}
		}
	}
	
	if cmd == nil {
		cmd = exec.CommandContext(ctx, ovpnBin, args...)
	}

	cmd.Dir = binDir
	PrepareHiddenCmd(cmd)

	a.mu.Lock()
	a.cmd = cmd
	a.tmpConfig = tmpPath
	a.tmpCreds = credsPath
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.lockFile = lockFilePath
	a.mu.Unlock()

	// Launch status reader goroutine to periodically update traffic statistics
	if statusFilePath != "" {
		go a.pollStatusFile(ctx, statusFilePath)
	}

	connectedCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		go a.streamLogs(stdout, "info", connectedCh, errCh)
	}
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go a.streamLogs(stderr, "warning", connectedCh, errCh)
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		if credsPath != "" {
			_ = os.Remove(credsPath)
		}
		return fmt.Errorf("failed to start openvpn process: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	a.log("info", fmt.Sprintf("OpenVPN process started (PID: %d)", cmd.Process.Pid))

	// Fix #2: Monitor process exit — fire onExit callback on unexpected crash
	go func() {
		waitErr := cmd.Wait()
		a.mu.Lock()
		wasRunning := a.running
		a.running = false
		onExit := a.onExit
		a.mu.Unlock()

		a.log("info", "OpenVPN process exited")

		if wasRunning && onExit != nil {
			if waitErr != nil {
				onExit(fmt.Errorf("openvpn process exited: %w", waitErr))
			} else {
				onExit(fmt.Errorf("openvpn process exited"))
			}
		}
	}()

	// Wait for OpenVPN handshake completion, explicit error, timeout, or cancellation
	select {
	case <-connectedCh:
		a.log("info", "OpenVPN tunnel established successfully!")
		return nil
	case opErr := <-errCh:
		_ = a.Stop()
		return opErr
	case <-time.After(25 * time.Second):
		_ = a.Stop()
		return fmt.Errorf("CONNECTION_TIMEOUT: اتصال به سرور پس از ۲۵ ثانیه برقرار نشد (پورت مسدود یا سرور خاموش است)")
	case <-ctx.Done():
		_ = a.Stop()
		return ctx.Err()
	}
}

func (a *OpenVPNAdapter) streamLogs(r io.Reader, defaultLevel string, connectedCh chan<- struct{}, errCh chan<- error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		level := defaultLevel
		lower := strings.ToLower(text)

		if strings.Contains(lower, "initialization sequence completed") {
			level = "info"
			select {
			case connectedCh <- struct{}{}:
			default:
			}
		} else if strings.Contains(lower, "auth_failed") || strings.Contains(lower, "password verification failed") {
			level = "error"
			a.log("error", "AUTHENTICATION_FAILED: نام کاربری یا رمز عبور اشتباه است یا اشتراک منقضی شده است (AUTH_FAILED)")
			select {
			case errCh <- fmt.Errorf("AUTH_FAILED: نام کاربری یا رمز عبور اشتباه است (AUTH_FAILED)"):
			default:
			}
		} else if strings.Contains(lower, "tls key negotiation failed") || strings.Contains(lower, "tls handshake failed") {
			level = "error"
			a.log("error", "ISP Filtering Detected: پورت یا سرور OpenVPN مسدود است یا پاسخی دریافت نشد.")
		} else if strings.Contains(lower, "cannot resolve host") {
			level = "error"
			a.log("error", "DNS Resolution Error: آدرس سرور OpenVPN ترجمه نشد.")
			select {
			case errCh <- fmt.Errorf("DNS_FAILED: دامنه سرور قابل دسترسی نیست"):
			default:
			}
		} else if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
			level = "error"
			if strings.Contains(lower, "--ca fails with") || strings.Contains(lower, "cannot find the file") {
				a.log("warning", "Hint: OpenVPN requires external certificate files. Embed them inside <ca>...</ca> in your .ovpn or copy to bin folder.")
			}
		} else if strings.Contains(lower, "warn") {
			level = "warning"
		}
		a.log(level, text)
	}
}

func (a *OpenVPNAdapter) Stop() error {
	a.mu.Lock()
	cmd := a.cmd
	tmpConfig := a.tmpConfig
	tmpCreds := a.tmpCreds
	lockFile := a.lockFile
	a.cmd = nil
	a.tmpConfig = ""
	a.tmpCreds = ""
	a.lockFile = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping OpenVPN process (PID: %d)...", pid))
		if lockFile != "" {
			_ = os.Remove(lockFile)
			time.Sleep(200 * time.Millisecond) // Give the script time to catch the deleted lockfile and kill the process gracefully
		}
		KillProcessTree(pid)
	}
	if tmpConfig != "" {
		_ = os.Remove(tmpConfig)
	}
	if tmpCreds != "" {
		_ = os.Remove(tmpCreds)
	}

	a.log("info", "OpenVPN stopped")
	return nil
}

func (a *OpenVPNAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return 0, 0, nil
	}
	return a.bytesRx, a.bytesTx, nil
}

func (a *OpenVPNAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

func (a *OpenVPNAdapter) pollStatusFile(ctx context.Context, statusPath string) {
	defer os.Remove(statusPath)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			running := a.running
			a.mu.RUnlock()
			if !running {
				return
			}

			data, err := os.ReadFile(statusPath)
			if err != nil {
				continue
			}

			lines := strings.Split(string(data), "\n")
			var rx, tx int64
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "TCP/UDP read bytes,") {
					var val int64
					if n, _ := fmt.Sscanf(line, "TCP/UDP read bytes,%d", &val); n == 1 {
						rx = val
					}
				} else if strings.HasPrefix(line, "TCP/UDP write bytes,") {
					var val int64
					if n, _ := fmt.Sscanf(line, "TCP/UDP write bytes,%d", &val); n == 1 {
						tx = val
					}
				} else if rx == 0 && strings.HasPrefix(line, "TUN/TAP write bytes,") {
					var val int64
					if n, _ := fmt.Sscanf(line, "TUN/TAP write bytes,%d", &val); n == 1 {
						rx = val
					}
				} else if tx == 0 && strings.HasPrefix(line, "TUN/TAP read bytes,") {
					var val int64
					if n, _ := fmt.Sscanf(line, "TUN/TAP read bytes,%d", &val); n == 1 {
						tx = val
					}
				}
			}

			if rx > 0 || tx > 0 {
				a.mu.Lock()
				a.bytesRx = rx
				a.bytesTx = tx
				a.mu.Unlock()
			}
		}
	}
}
