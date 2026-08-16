package protocols

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/sysproxy"
)

type SingBoxAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	tmpConfig   string
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	lastError   string
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

// generateClashSecret produces a random 16-byte hex secret for the Clash API.
func generateClashSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *SingBoxAdapter) Start(ctx context.Context, configContent string) error {
	sbBin, err := findBinary("sing-box")
	if err != nil {
		return fmt.Errorf("sing-box.exe not found. Please place sing-box.exe in bin folder")
	}

	// Fix #14: Inject a random Clash API secret into the config JSON
	configContent = injectClashSecret(configContent)

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
	cmd := exec.CommandContext(ctx, sbBin, "run", "-c", tmpPath, "-D", binDir)
	cmd.Dir = binDir
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+";"+os.Getenv("PATH"),
	)
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
		return fmt.Errorf("failed to start sing-box process: %w", err)
	}

	a.mu.Lock()
	a.cmd = cmd
	a.tmpConfig = tmpPath
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.lastError = ""
	a.mu.Unlock()

	a.log("info", fmt.Sprintf("Sing-box core started (PID: %d)", cmd.Process.Pid))

	// Fix #2: Monitor process exit — fire onExit callback on unexpected crash
	go func() {
		waitErr := cmd.Wait()
		a.mu.Lock()
		wasRunning := a.running
		a.running = false
		onExit := a.onExit
		a.mu.Unlock()

		a.log("info", "Sing-box process exited")

		if wasRunning && onExit != nil {
			if waitErr != nil {
				onExit(fmt.Errorf("sing-box process exited unexpectedly: %w", waitErr))
			} else {
				onExit(fmt.Errorf("sing-box process exited unexpectedly"))
			}
		}
	}()

	// Probe local mixed port 10808
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(300 * time.Millisecond)
			conn, err := net.DialTimeout("tcp", "127.0.0.1:10808", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				break
			}
		}
	}()

	statsCtx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.statsCancel = cancel
	a.mu.Unlock()
	go a.pollClashStats(statsCtx)

	return nil
}

// injectClashSecret adds a random secret to the clash_api config block if present.
func injectClashSecret(configContent string) string {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configContent), &cfg); err != nil {
		return configContent
	}
	exp, ok := cfg["experimental"].(map[string]interface{})
	if !ok {
		return configContent
	}
	clashAPI, ok := exp["clash_api"].(map[string]interface{})
	if !ok {
		return configContent
	}
	clashAPI["secret"] = ""
	exp["clash_api"] = clashAPI
	cfg["experimental"] = exp
	if b, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		return string(b)
	}
	return configContent
}

func (a *SingBoxAdapter) pollClashStats(ctx context.Context) {
	// Fix #3: Use a proper timeout to prevent goroutine leak
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:9090/traffic", nil)
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
	a.cmd = nil
	a.tmpConfig = ""
	a.running = false
	// Fix #2: Clear onExit before stopping — prevent false alarm on intentional stop
	a.onExit = nil
	a.mu.Unlock()

	_ = sysproxy.ClearSystemProxy()

	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		a.log("info", fmt.Sprintf("Stopping Sing-box core (PID: %d)...", pid))
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
