package protocols

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"nexvpn/internal/config"
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
		return fmt.Errorf("OpenVPN binary 'openvpn.exe' not found. Please install OpenVPN or place openvpn.exe in bin folder")
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
	}

	if credsPath != "" {
		args = append(args, "--auth-user-pass", credsPath)
	}

	cmd := exec.CommandContext(ctx, ovpnBin, args...)
	cmd.Dir = binDir
	PrepareHiddenCmd(cmd)

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		go a.streamLogs(stdout, "info")
	}
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go a.streamLogs(stderr, "warning")
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		if credsPath != "" {
			_ = os.Remove(credsPath)
		}
		return fmt.Errorf("failed to start openvpn process: %w", err)
	}

	a.mu.Lock()
	a.cmd = cmd
	a.tmpConfig = tmpPath
	a.tmpCreds = credsPath
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.mu.Unlock()

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
				onExit(fmt.Errorf("openvpn process exited unexpectedly: %w", waitErr))
			} else {
				onExit(fmt.Errorf("openvpn process exited unexpectedly"))
			}
		}
	}()

	return nil
}

func (a *OpenVPNAdapter) streamLogs(r io.Reader, defaultLevel string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		level := defaultLevel
		lower := strings.ToLower(text)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
			level = "error"
			if strings.Contains(lower, "--ca fails with") || strings.Contains(lower, "cannot find the file") {
				a.log("warning", "Hint: OpenVPN requires external certificate files. Embed them inside <ca>...</ca> in your .ovpn or copy to bin folder.")
			} else if strings.Contains(lower, "tls key negotiation failed") || strings.Contains(lower, "tls handshake failed") {
				a.log("error", "ISP Filtering Detected: OpenVPN UDP port is filtered/dropped by your ISP. Please use an OpenVPN TCP (port 443) profile, or switch to VLESS Reality / WARP.")
			}
		} else if strings.Contains(lower, "warn") {
			level = "warning"
		} else if strings.Contains(lower, "initialization sequence completed") {
			level = "info"
		}
		a.log(level, text)
	}
}

func (a *OpenVPNAdapter) Stop() error {
	a.mu.Lock()
	cmd := a.cmd
	tmpConfig := a.tmpConfig
	tmpCreds := a.tmpCreds
	a.cmd = nil
	a.tmpConfig = ""
	a.tmpCreds = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping OpenVPN process (PID: %d)...", pid))
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
