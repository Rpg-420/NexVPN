package protocols

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
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
	lastError string
	exitCh    chan struct{}
	exitOnce  sync.Once
	// activeDriver is the OpenVPN driver that actually carried the tunnel:
	// "wintun" or "tap-windows6". Start() prefers Wintun but silently retries
	// with tap-windows6 when Wintun is unusable — for instance when every Wintun
	// adapter on the machine is already locked, which OpenVPN reports as
	// WINTUN_IN_USE. Without recording which one won, the engine announced
	// "Full-System Routing with Wintun" over a TAP tunnel, so the log and the
	// adapter actually in use (Ethernet 2) contradicted each other.
	activeDriver string
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

// ActiveDriver reports the driver the running tunnel is actually using, or ""
// when no driver was passed (non-Windows, or the OS default). Callers must treat
// an empty value as "unknown" rather than assuming Wintun.
func (a *OpenVPNAdapter) ActiveDriver() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeDriver
}

func (a *OpenVPNAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "openvpn")
	}
}

// fail records a terminal protocol error in exactly one canonical place:
// it stores the message for LastError(), logs it once, and pushes it onto
// errCh (non-blocking so a stalled consumer can never wedge the log reader).
//
// Error messages MUST be English, machine-readable, and prefixed with a stable
// code ("AUTH_FAILED: ...", "TLS_FAILED: ...", "DNS_FAILED: ...",
// "WINTUN_IN_USE: ..."). The UI maps the prefix to a localized string, so
// hardcoding a translated sentence here both breaks that mapping and produces
// contradictory output. This helper exists to make the "one event, one message"
// rule the path of least resistance.
func (a *OpenVPNAdapter) fail(errCh chan<- error, msg string) {
	a.mu.Lock()
	a.lastError = msg
	a.mu.Unlock()
	a.log("error", msg)
	select {
	case errCh <- errors.New(msg):
	default:
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
		// NexVPN owns the OpenVPN Wintun adapter selection. A stale profile-level
		// device name can point at another VPN application's adapter and produce
		// WINTUN_IN_USE before OpenVPN can initialize its tunnel.
		if strings.HasPrefix(lowerLine, "dev-node") || strings.HasPrefix(lowerLine, "windows-driver") {
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

type WindowsAdapterInfo struct {
	Name   string
	GUID   string
	Driver string
}

func queryOpenVPNAdapters(ovpnBin string) []WindowsAdapterInfo {
	cmd := exec.Command(ovpnBin, "--show-adapters")
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var result []WindowsAdapterInfo
	re := regexp.MustCompile(`'([^']+)'\s+\{([^}]+)\}\s+([a-zA-Z0-9\-_]+)`)
	for _, line := range strings.Split(string(out), "\n") {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			result = append(result, WindowsAdapterInfo{
				Name:   matches[1],
				GUID:   matches[2],
				Driver: strings.ToLower(matches[3]),
			})
		}
	}
	return result
}

// How long the Wintun adapter pool is given to drain before OpenVPN is started
// on Wintun.
//
// Windows releases a Wintun binding asynchronously: the previous core process can
// be gone for several seconds while the adapter is still reported as bound, and
// that is precisely the state in which OpenVPN prints "All Wintun network
// adapters are currently locked or disabled" and dies — after a full TLS
// handshake, because the adapter is only opened once the tunnel is negotiated.
// A fixed sleep could only guess at the delay; a poll knows when it is done, and
// costs nothing at all when there was nothing to wait for.
const (
	wintunPoolPollInterval = 500 * time.Millisecond
	wintunPoolWaitBudget   = 6 * time.Second
)

// waitForNexVPNWintunAdaptersGone removes leftover NexVPN Wintun adapters and
// waits, within a bound, for the pool to be free again. It reports whether the
// pool looks usable for a Wintun-backed OpenVPN connection.
//
// Only NexVPN's own adapters are touched; a third-party Wintun adapter belongs
// to another product and is never removed here. Off Windows the scan always
// reports "nothing of ours", so this returns immediately.
//
// The wait is for a *live* adapter, not for any trace of one. A ghost — a
// registry binding with no device behind it — is what a killed TUN core leaves
// on most machines, and it holds no handle: waiting for it can never succeed,
// and the 2026-10-12 log shows exactly that, 8.5 s burned per OpenVPN connect
// followed by the same tap-windows6 fallback. Ghosts are still cleared (they do
// block a core that asks for that exact name), they just no longer decide the
// driver.
func waitForNexVPNWintunAdaptersGone(log func(level, msg string)) bool {
	if NexVPNWintunAdapterPresent(ScanWintunGhosts()) {
		log("info", "Leftover NexVPN Wintun adapter detected; clearing it before the Wintun attempt")
		ClearStaleNexVPNWintunAdapters()
	}

	deadline := time.Now().Add(wintunPoolWaitBudget)
	for {
		if !NexVPNWintunAdapterLive(ScanWintunGhosts()) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(wintunPoolPollInterval)
	}
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
		credsFile, err := os.CreateTemp(ensureNexVPNTempDir(), "openvpn-creds-*.txt")
		if err == nil {
			credsPath = credsFile.Name()
			_, _ = fmt.Fprintf(credsFile, "%s\n%s\n", username, password)
			_ = credsFile.Close()
		}
	}

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
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		if credsPath != "" {
			_ = os.Remove(credsPath)
		}
		return err
	}
	_ = tmpFile.Close()

	binDir := filepath.Dir(ovpnBin)

	// Ensure 64-bit wintun.dll is available in binDir
	wintunPath := filepath.Join(binDir, "wintun.dll")
	if fi, statErr := os.Stat(wintunPath); os.IsNotExist(statErr) || (fi != nil && fi.Size() > 500000) {
		if srcWintun, findErr := findBinary("wintun.dll"); findErr == nil {
			if data, readErr := os.ReadFile(srcWintun); readErr == nil && len(data) < 500000 {
				_ = os.WriteFile(wintunPath, data, 0644)
			}
		}
	}

	if runtime.GOOS == "windows" {
		if !sysproc.IsAdmin() {
			a.log("error", "OpenVPN Layer-3 tunnel requires Administrator privileges on Windows to create or open network adapters.")
			return fmt.Errorf("ADMIN_REQUIRED: OpenVPN requires administrator privileges to create network adapters. Please run NexVPN as Administrator (Run as administrator)")
		}

		adapters := queryOpenVPNAdapters(ovpnBin)
		var tapAdapterName string
		for _, ad := range adapters {
			if ad.Driver == "tap-windows6" && tapAdapterName == "" {
				// Check if the TAP adapter is enabled in Windows
				checkCmd := exec.Command("netsh", "interface", "show", "interface", ad.Name)
				checkCmd.SysProcAttr = sysproc.HideWindowAttr()
				out, err := checkCmd.Output()
				if err == nil && !strings.Contains(strings.ToLower(string(out)), "disabled") {
					tapAdapterName = ad.Name
				}
			}
		}

		// When running with Administrator privileges on Windows, OpenVPN 2.5 with Wintun
		// dynamically manages its own adapter. Do not pass an explicit non-existent devNode name.
		//
		// Before that attempt, clear any leftover NexVPN Wintun adapter. OpenVPN
		// does not create a named adapter of ours, but Wintun adapters are a
		// shared, machine-wide pool: one of ours still bound from a core that has
		// not fully exited is enough to make OpenVPN print "All Wintun network
		// adapters on this system are currently in use or disabled" and quietly
		// drop to tap-windows6. The engine's TUN prep runs earlier in the
		// pipeline, but a core killed in between can re-create the binding, so the
		// check is repeated here, immediately before the Wintun attempt. Scoped to
		// our own names only — a third-party Wintun adapter is never touched.
		wintunUsable := waitForNexVPNWintunAdaptersGone(a.log)

		primaryDriver := "wintun"
		primaryDevNode := ""
		fallbackDriver := ""
		fallbackDevNode := ""

		if tapAdapterName != "" {
			fallbackDriver = "tap-windows6"
			fallbackDevNode = tapAdapterName
		}

		if !wintunUsable && fallbackDriver != "" {
			// A leftover of ours is still holding the pool, and it will not let go
			// in the next few seconds: Windows releases a Wintun binding only after
			// the previous owner's handle is gone. Starting on Wintun anyway costs a
			// complete TLS handshake first — the adapter is opened after the server
			// is authenticated — and then the same connection is negotiated again on
			// TAP. Choosing the driver that will actually work up front saves the
			// round trip and says why in the log.
			a.log("warning", fmt.Sprintf(
				"A NexVPN Wintun adapter is still holding the adapter pool after cleanup; starting directly on %s (%s) instead of repeating a handshake that cannot finish",
				fallbackDriver, fallbackDevNode))
			primaryDriver = fallbackDriver
			primaryDevNode = fallbackDevNode
			fallbackDriver = ""
			fallbackDevNode = ""
		}

		a.log("info", fmt.Sprintf("OpenVPN driver selection: %s", primaryDriver))
		// `keepTempFiles` is true whenever a fallback might reuse this attempt's
		// temp files. It must be: `launchDriverInstance` tears the process down on
		// failure, and a plain `Stop()` also deletes tmpConfig/tmpCreds. Deleting
		// them before the TAP retry made every fallback die instantly with
		// `Options error: Error opening configuration file: ...\openvpn-NNNN.ovpn`,
		// which looks like a profile problem but is really us removing the file
		// out from under OpenVPN.
		runErr := a.launchDriverInstance(ctx, ovpnBin, tmpPath, credsPath, binDir, primaryDriver, primaryDevNode, fallbackDriver != "")
		if runErr != nil && fallbackDriver != "" {
			errStr := strings.ToLower(runErr.Error())
			if strings.Contains(errStr, "wintun") || strings.Contains(errStr, "adapter") || strings.Contains(errStr, "createfile") || strings.Contains(errStr, "ring buffer") {
				a.log("warning", fmt.Sprintf("OpenVPN %s driver failed (%v). Auto-switching to fallback %s driver (%s)...", primaryDriver, runErr, fallbackDriver, fallbackDevNode))
				time.Sleep(400 * time.Millisecond)
				fbErr := a.launchDriverInstance(ctx, ovpnBin, tmpPath, credsPath, binDir, fallbackDriver, fallbackDevNode, false)
				if fbErr != nil {
					// The fallback was the last attempt, so nothing will reuse these
					// files. Remove them explicitly: the inner teardown already
					// cleared a.tmpConfig/a.tmpCreds (that is what preserved them for
					// the retry), so a later Stop() would no longer know they exist.
					_ = os.Remove(tmpPath)
					if credsPath != "" {
						_ = os.Remove(credsPath)
					}
				}
				return fbErr
			}
			// Not a driver fault, so the TAP retry cannot help. Clean up here
			// instead of leaving the pair behind in the temp directory.
			_ = os.Remove(tmpPath)
			if credsPath != "" {
				_ = os.Remove(credsPath)
			}
		}
		return runErr
	}

	return a.launchDriverInstance(ctx, ovpnBin, tmpPath, credsPath, binDir, "", "", false)
}

// launchDriverInstance runs one OpenVPN process for the given driver and blocks
// until it is connected, failed, or timed out.
//
// keepTempFiles controls what happens to tmpPath/credsPath on every failure path
// inside: when false they are deleted (the usual case), when true they are left
// on disk so a caller can immediately retry with the same files. `a.Stop()` is
// never used here for that reason — it always deletes them.
func (a *OpenVPNAdapter) launchDriverInstance(ctx context.Context, ovpnBin, tmpPath, credsPath, binDir, driver, devNode string, keepTempFiles bool) error {
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

	if runtime.GOOS == "windows" {
		if driver != "" {
			args = append(args, "--windows-driver", driver)
		}
		if devNode != "" {
			args = append(args, "--dev-node", devNode)
		}
		args = append(args,
			"--pull-filter", "ignore", "route-ipv6",
			"--pull-filter", "ignore", "ifconfig-ipv6",
			"--pull-filter", "ignore", "tun-ipv6",
			"--block-ipv6",
		)
	}

	statusFile, err := os.CreateTemp(ensureNexVPNTempDir(), "openvpn-status-*.log")
	var statusFilePath string
	if err == nil {
		statusFilePath = statusFile.Name()
		_ = statusFile.Close()
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
				_ = lockFile.Close()
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

	exitCh := make(chan struct{})

	a.mu.Lock()
	a.cmd = cmd
	a.tmpConfig = tmpPath
	a.tmpCreds = credsPath
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.lockFile = lockFilePath
	a.lastError = ""
	a.exitCh = exitCh
	a.exitOnce = sync.Once{}
	if driver != "" {
		// Recorded before the wait so a failed Wintun attempt followed by a
		// successful TAP retry ends up reporting tap-windows6, which is the
		// driver the tunnel is really on.
		a.activeDriver = driver
	}
	a.mu.Unlock()

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
		if !keepTempFiles {
			_ = os.Remove(tmpPath)
			if credsPath != "" {
				_ = os.Remove(credsPath)
			}
		}
		return fmt.Errorf("failed to start openvpn process: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	a.log("info", fmt.Sprintf("OpenVPN process started (PID: %d, Driver: %s)", cmd.Process.Pid, driver))

	go func() {
		waitErr := cmd.Wait()
		a.mu.Lock()
		wasRunning := a.running
		a.running = false
		onExit := a.onExit
		a.mu.Unlock()

		a.exitOnce.Do(func() { close(exitCh) })

		a.log("info", "OpenVPN process exited")

		if wasRunning && onExit != nil {
			if waitErr != nil {
				onExit(fmt.Errorf("openvpn process exited: %w", waitErr))
			} else {
				onExit(fmt.Errorf("openvpn process exited"))
			}
		}
	}()

	select {
	case <-connectedCh:
		a.log("info", "OpenVPN tunnel established successfully!")
		return nil
	case opErr := <-errCh:
		_ = a.stopInternal(!keepTempFiles)
		return opErr
	case <-exitCh:
		a.mu.RLock()
		lastErr := a.lastError
		a.mu.RUnlock()
		_ = a.stopInternal(!keepTempFiles)
		if lastErr == "" {
			lastErr = "OpenVPN process terminated during startup"
		}
		return fmt.Errorf("%s", lastErr)
	case <-time.After(25 * time.Second):
		_ = a.stopInternal(!keepTempFiles)
		return fmt.Errorf("CONNECTION_TIMEOUT: OpenVPN connection timed out after 25s (server offline or port blocked)")
	case <-ctx.Done():
		_ = a.stopInternal(!keepTempFiles)
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
		} else if strings.Contains(lower, "all wintun adapters on this system are currently in use or disabled") {
			level = "error"
			a.fail(errCh, "WINTUN_IN_USE: All Wintun network adapters are currently locked or disabled")
		} else if strings.Contains(lower, "auth_failed") || strings.Contains(lower, "password verification failed") {
			level = "error"
			a.fail(errCh, "AUTH_FAILED: Authentication failed (invalid credentials or expired subscription)")
		} else if strings.Contains(lower, "tls key negotiation failed") || strings.Contains(lower, "tls handshake failed") {
			level = "error"
			a.fail(errCh, "TLS_FAILED: TLS key negotiation failed (server unreachable or port throttled)")
		} else if strings.Contains(lower, "cannot resolve host") {
			level = "error"
			a.fail(errCh, "DNS_FAILED: Cannot resolve OpenVPN server hostname")
		} else if strings.Contains(lower, "options error") {
			level = "error"
			a.mu.Lock()
			a.lastError = text
			a.mu.Unlock()
			select {
			case errCh <- fmt.Errorf("OPENVPN_CONFIG_ERROR: %s", text):
			default:
			}
		} else if strings.Contains(lower, "exiting due to fatal error") {
			level = "error"
			a.mu.Lock()
			if a.lastError == "" {
				a.lastError = text
			}
			a.mu.Unlock()
			select {
			case errCh <- fmt.Errorf("FATAL_ERROR: %s", text):
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

// Stop tears the adapter down and removes its temporary files.
func (a *OpenVPNAdapter) Stop() error {
	return a.stopInternal(true)
}

// stopInternal is Stop with the temp-file removal made optional.
//
// removeTempFiles=false exists for exactly one caller: the Wintun→TAP driver
// fallback in Start(), which must kill the failed process but hand the *same*
// tmpConfig/credsPath to the retry. Clearing a.tmpConfig/a.tmpCreds is correct
// either way — it stops a later Stop() from double-removing a path that a retry
// has since overwritten — which is why the fallback removes the files itself
// once the retry has also failed.
func (a *OpenVPNAdapter) stopInternal(removeTempFiles bool) error {
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
		if err := KillProcessTree(pid); err != nil {
			// A live OpenVPN wrapper still holds the config and the tun
			// device, so the caller must not be told the stop succeeded.
			a.log("error", fmt.Sprintf("OpenVPN process (PID: %d) did not stop: %v", pid, err))
		}
	}
	if removeTempFiles {
		if tmpConfig != "" {
			_ = os.Remove(tmpConfig)
		}
		if tmpCreds != "" {
			_ = os.Remove(tmpCreds)
		}
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
	defer func() { _ = os.Remove(statusPath) }()
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
