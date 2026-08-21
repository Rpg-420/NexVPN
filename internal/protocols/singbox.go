package protocols

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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
	"nexvpn/internal/sysproxy"
)

type SingBoxAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	tmpConfig   string
	clashPort   int
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	lastError   string
	lockFile    string
}

func NewSingBoxAdapter() *SingBoxAdapter {
	return &SingBoxAdapter{}
}

func (a *SingBoxAdapter) Name() config.Protocol {
	return config.ProtocolSingBox
}

func (a *SingBoxAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	a.mu.Unlock()
}

// SetOnExit registers a callback invoked when the sing-box process exits unexpectedly.
func (a *SingBoxAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.Lock()
	a.onExit = fn
	a.mu.Unlock()
}

func (a *SingBoxAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "sing-box")
	}
}

// getFreeLocalPort finds an available TCP port for Clash API to prevent 9090 port conflicts
func getFreeLocalPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 19090
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func (a *SingBoxAdapter) Start(ctx context.Context, configContent string) error {
	sbBin, err := findBinary("sing-box")
	if err != nil {
		return fmt.Errorf("Sing-box core binary not found. Please ensure 'sing-box' binary is in bin folder")
	}

	// Dynamically allocate a free local port for clash_api
	port := getFreeLocalPort()
	a.mu.Lock()
	a.clashPort = port
	a.mu.Unlock()

	configContent = injectClashConfig(configContent, port)

	// Fix #15: Use dedicated secure temp dir
	tmpDir := ensureNexVPNTempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "singbox-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(configContent); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()

	binDir := filepath.Dir(sbBin)
	cmdArgs := []string{"run", "-c", tmpPath, "-D", binDir}
	var cmd *exec.Cmd
	var lockFilePath string

	u, errUser := user.Current()
	isRoot := errUser == nil && u.Uid == "0"

	if runtime.GOOS == "linux" && !isRoot && !hasNetCap(sbBin) && strings.Contains(configContent, `"type": "tun"`) {
		if _, err := exec.LookPath("pkexec"); err == nil {
			lockFile, err := os.CreateTemp(ensureNexVPNTempDir(), "singbox-lock-*.lock")
			if err == nil {
				lockFilePath = lockFile.Name()
				lockFile.Close()
				script := `
					"$1" run -c "$2" -D "$3" &
					pid=$!
					while kill -0 $pid 2>/dev/null && [ -f "$4" ]; do
						sleep 0.5
					done
					kill -9 $pid 2>/dev/null
					exit 0
				`
				args := []string{"sh", "-c", script, "sh", sbBin, tmpPath, binDir, lockFilePath}
				cmd = exec.CommandContext(ctx, "pkexec", args...)
			}
		}
	}

	if cmd == nil {
		cmd = exec.CommandContext(ctx, sbBin, cmdArgs...)
	}

	cmd.Dir = binDir
	cmd.Env = append(os.Environ(),
		"ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true",
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DBUS_SYSTEM_BUS_ADDRESS=unix:path=/dev/null", // Prevent sing-box from triggering NetworkManager Polkit route prompt
	)
	PrepareHiddenCmd(cmd)

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		go a.streamLogs(stdout, "info")
	}
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go a.streamLogs(stderr, "error")
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	a.mu.Lock()
	a.cmd = cmd
	a.running = true
	a.tmpConfig = tmpPath
	a.bytesRx = 0
	a.bytesTx = 0
	a.lastError = ""
	a.lockFile = lockFilePath
	a.mu.Unlock()

	a.log("info", fmt.Sprintf("Sing-box core started (PID: %d)", cmd.Process.Pid))

	statsCtx, statsCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.statsCancel = statsCancel
	a.mu.Unlock()
	go a.pollClashStats(statsCtx)

	go a.waitForExit(cmd, tmpPath)

	return nil
}

func (a *SingBoxAdapter) waitForExit(cmd *exec.Cmd, tmpPath string) {
	err := cmd.Wait()

	a.mu.Lock()
	wasRunning := a.running
	a.running = false
	a.cmd = nil
	a.mu.Unlock()

	_ = os.Remove(tmpPath)

	if wasRunning {
		a.log("info", "Sing-box process exited")
		if err != nil {
			a.mu.RLock()
			onExit := a.onExit
			a.mu.RUnlock()
			if onExit != nil {
				onExit(fmt.Errorf("sing-box process exited unexpectedly: %w", err))
			}
		}
	}
}

// injectClashConfig injects the dynamic port and secret to clash_api
func injectClashConfig(configContent string, port int) string {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configContent), &cfg); err != nil {
		return configContent
	}
	exp, ok := cfg["experimental"].(map[string]interface{})
	if !ok {
		exp = make(map[string]interface{})
	}
	clashAPI, ok := exp["clash_api"].(map[string]interface{})
	if !ok {
		clashAPI = make(map[string]interface{})
	}
	clashAPI["external_controller"] = fmt.Sprintf("127.0.0.1:%d", port)
	clashAPI["secret"] = ""
	exp["clash_api"] = clashAPI
	cfg["experimental"] = exp
	if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		return string(b)
	}
	return configContent
}

func (a *SingBoxAdapter) pollClashStats(ctx context.Context) {
	client := &http.Client{Timeout: 5 * time.Second}

	a.mu.RLock()
	cPort := a.clashPort
	a.mu.RUnlock()
	if cPort <= 0 {
		cPort = 19090
	}
	trafficURL := fmt.Sprintf("http://127.0.0.1:%d/traffic", cPort)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", trafficURL, nil)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
				return
			default:
			}

			line := scanner.Bytes()
			var traffic struct {
				Up   int64 `json:"up"`
				Down int64 `json:"down"`
			}
			if err := json.Unmarshal(line, &traffic); err == nil {
				a.mu.Lock()
				a.bytesRx += traffic.Down
				a.bytesTx += traffic.Up
				a.mu.Unlock()
			}
		}
		resp.Body.Close()
		time.Sleep(1 * time.Second)
	}
}

func (a *SingBoxAdapter) streamLogs(r io.Reader, defaultLevel string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		level := defaultLevel
		lower := strings.ToLower(text)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
			level = "error"
			a.mu.Lock()
			a.lastError = text
			a.mu.Unlock()
		} else if strings.Contains(lower, "warn") {
			level = "warning"
		} else if strings.Contains(lower, "info") {
			level = "info"
		}
		a.log(level, text)
	}
}

func (a *SingBoxAdapter) Stop() error {
	a.mu.Lock()
	if a.statsCancel != nil {
		a.statsCancel()
		a.statsCancel = nil
	}
	cmd := a.cmd
	tmpConfig := a.tmpConfig
	lockFile := a.lockFile
	a.cmd = nil
	a.tmpConfig = ""
	a.lockFile = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	_ = sysproxy.ClearSystemProxy()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping Sing-box core (PID: %d)...", pid))
		if lockFile != "" {
			_ = os.Remove(lockFile)
			time.Sleep(200 * time.Millisecond) // Give the script time to catch the deleted lockfile and kill the process gracefully
		}
		KillProcessTree(pid)
	}
	if tmpConfig != "" {
		_ = os.Remove(tmpConfig)
	}

	a.log("info", "Sing-box stopped")
	return nil
}

func (a *SingBoxAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.running {
		return 0, 0, nil
	}
	return a.bytesRx, a.bytesTx, nil
}

func (a *SingBoxAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
