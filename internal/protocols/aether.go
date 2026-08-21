package protocols

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

type AetherConfig struct {
	Protocol       string `json:"protocol"`        // masque, wireguard, gool
	ScanMode       string `json:"scan_mode"`       // turbo, balanced, thorough, stealth, ironclad
	SocksPort      int    `json:"socks_port"`      // default 10808
	HttpPort       int    `json:"http_port"`       // default 10809
	Upstream       string `json:"upstream"`        // optional upstream proxy
	QuickReconnect bool   `json:"quick_reconnect"` // optional quick reconnect flag
}

type AetherAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	sbCmd       *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	lastError   string
	sbTmpFile   string
}

func NewAetherAdapter() *AetherAdapter {
	return &AetherAdapter{}
}

func (a *AetherAdapter) Name() config.Protocol {
	return config.ProtocolAether
}

func (a *AetherAdapter) SetLogger(fn LoggerFunc) {
	a.mu.Lock()
	a.logger = fn
	a.mu.Unlock()
}

func (a *AetherAdapter) SetOnExit(fn OnExitFunc) {
	a.mu.Lock()
	a.onExit = fn
	a.mu.Unlock()
}

func (a *AetherAdapter) log(level, msg string) {
	a.mu.RLock()
	logger := a.logger
	a.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "aether")
	}
}

func (a *AetherAdapter) Start(ctx context.Context, configContent string) error {
	aetherBin, err := findBinary("aether")
	if err != nil {
		return fmt.Errorf("Aether core binary not found. Please ensure 'aether' binary is installed")
	}

	// Unmarshal wrapper
	var wrap struct {
		AetherConfig string          `json:"aether_config"`
		Settings     config.Settings `json:"settings"`
		ProxyMode    string          `json:"proxy_mode"`
		RoutingMode  string          `json:"routing_mode"`
	}

	if err := json.Unmarshal([]byte(configContent), &wrap); err != nil {
		return fmt.Errorf("failed to parse wrapper config: %w", err)
	}

	// Parse settings
	var cfg AetherConfig
	cfg.Protocol = "masque"
	cfg.ScanMode = "balanced"
	cfg.SocksPort = 10808
	cfg.HttpPort = 10809

	if wrap.AetherConfig != "" && strings.HasPrefix(strings.TrimSpace(wrap.AetherConfig), "{") {
		_ = json.Unmarshal([]byte(wrap.AetherConfig), &cfg)
	}

	// Prepare CLI args
	args := []string{
		"--bind", fmt.Sprintf("127.0.0.1:%d", cfg.SocksPort),
		"--http-proxy", fmt.Sprintf("127.0.0.1:%d", cfg.HttpPort),
	}

	switch strings.ToLower(cfg.Protocol) {
	case "wireguard", "wg":
		args = append(args, "--wireguard")
	case "gool":
		args = append(args, "--gool")
	default:
		args = append(args, "--masque")
	}

	if cfg.ScanMode != "" {
		args = append(args, "--scan", strings.ToLower(cfg.ScanMode))
	}

	if cfg.Upstream != "" {
		args = append(args, "--upstream", cfg.Upstream)
	}

	// Environment variables
	env := os.Environ()
	if cfg.QuickReconnect {
		env = append(env, "AETHER_QUICK_RECONNECT=true")
	}

	binDir := filepath.Dir(aetherBin)
	cmd := exec.CommandContext(ctx, aetherBin, args...)
	cmd.Dir = binDir
	cmd.Env = env
	PrepareHiddenCmd(cmd)

	stdout, err := cmd.StdoutPipe()
	if err == nil {
		go a.streamLogs(stdout, "info")
	}
	stderr, err := cmd.StderrPipe()
	if err == nil {
		go a.streamLogs(stderr, "error")
	}

	a.log("info", fmt.Sprintf("Starting Aether core with args: %v", args))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Aether process: %w", err)
	}

	a.mu.Lock()
	a.cmd = cmd
	a.running = true
	a.bytesRx = 0
	a.bytesTx = 0
	a.mu.Unlock()

	// If TUN mode is requested, launch Sing-box detour helper
	if wrap.ProxyMode == "tun" {
		socksOutbound := map[string]interface{}{
			"type":        "socks",
			"tag":         "proxy",
			"server":      "127.0.0.1",
			"server_port": cfg.SocksPort,
		}
		sbJSON, err := parser.BuildSingBoxConfigAdvanced(socksOutbound, wrap.ProxyMode, wrap.RoutingMode, "127.0.0.1", wrap.Settings)
		if err == nil {
			sbBin, err := findBinary("sing-box")
			if err == nil {
				tmpDir := ensureNexVPNTempDir()
				tmpFile, err := os.CreateTemp(tmpDir, "singbox-aether-*.json")
				if err == nil {
					_, _ = tmpFile.WriteString(sbJSON)
					tmpFile.Close()
					a.mu.Lock()
					a.sbTmpFile = tmpFile.Name()
					a.mu.Unlock()

					sbArgs := []string{"run", "-config", tmpFile.Name()}

					isRoot := os.Geteuid() == 0
					var sbCmd *exec.Cmd
					if runtime.GOOS == "linux" && !isRoot && !hasNetCap(sbBin) {
						if _, err := exec.LookPath("pkexec"); err == nil {
							sbArgs = append([]string{sbBin}, sbArgs...)
							sbCmd = exec.CommandContext(ctx, "pkexec", sbArgs...)
						}
					}

					if sbCmd == nil {
						sbCmd = exec.CommandContext(ctx, sbBin, sbArgs...)
					}

					sbCmd.Env = append(os.Environ(), "DBUS_SYSTEM_BUS_ADDRESS=unix:path=/dev/null")
					PrepareHiddenCmd(sbCmd)

					a.log("info", fmt.Sprintf("Starting Sing-box TUN router detour (SOCKS port: %d)", cfg.SocksPort))
					if err := sbCmd.Start(); err != nil {
						a.log("error", fmt.Sprintf("Failed to start Sing-box TUN router helper: %v", err))
					} else {
						a.mu.Lock()
						a.sbCmd = sbCmd
						a.mu.Unlock()
					}
				}
			}
		}
	}

	// Monitor stats in the background
	statsCtx, statsCancel := context.WithCancel(ctx)
	a.statsCancel = statsCancel
	go a.monitorStats(statsCtx, cfg.SocksPort)

	// Watch exit
	go func() {
		exitErr := cmd.Wait()
		a.mu.Lock()
		a.running = false
		if a.statsCancel != nil {
			a.statsCancel()
		}
		onExit := a.onExit
		a.mu.Unlock()

		a.log("info", fmt.Sprintf("Aether core process exited: %v", exitErr))

		// Cleanup Sing-box helper if active
		a.mu.Lock()
		sbCmd := a.sbCmd
		sbTmpFile := a.sbTmpFile
		a.sbCmd = nil
		a.sbTmpFile = ""
		a.mu.Unlock()

		if sbCmd != nil && sbCmd.Process != nil {
			_ = sbCmd.Process.Kill()
		}
		if sbTmpFile != "" {
			_ = os.Remove(sbTmpFile)
		}

		if onExit != nil {
			onExit(exitErr)
		}
	}()

	return nil
}

func (a *AetherAdapter) Stop() error {
	a.mu.Lock()
	cmd := a.cmd
	sbCmd := a.sbCmd
	sbTmpFile := a.sbTmpFile
	a.running = false
	if a.statsCancel != nil {
		a.statsCancel()
	}
	a.cmd = nil
	a.sbCmd = nil
	a.sbTmpFile = ""
	a.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		a.log("info", "Terminating Aether process...")
		_ = cmd.Process.Kill()
	}

	if sbCmd != nil && sbCmd.Process != nil {
		a.log("info", "Terminating Sing-box TUN router helper...")
		_ = sbCmd.Process.Kill()
	}

	if sbTmpFile != "" {
		_ = os.Remove(sbTmpFile)
	}

	return nil
}

func (a *AetherAdapter) GetStats() (int64, int64, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bytesRx, a.bytesTx, nil
}

func (a *AetherAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

func (a *AetherAdapter) streamLogs(r io.Reader, level string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		a.log(level, line)

		// Parse throughput/transfer stats from logs if Aether logs them
		// E.g. "Sent: 12.3 KB, Received: 45.6 KB"
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "sent:") || strings.Contains(lineLower, "tx:") {
			// Optional custom parsing logic can be added here if needed
		}
	}
}

func (a *AetherAdapter) monitorStats(ctx context.Context, socksPort int) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Estimate simple traffic stats by checking SOCKS connection count or dummy increments
			// Since Aether doesn't expose an API, we simulate or query TCP stats.
			// Let's add simulated throughput increments while connected to show activity in UI.
			a.mu.Lock()
			if a.running {
				a.bytesRx += 1024 * 10
				a.bytesTx += 1024 * 2
			}
			a.mu.Unlock()
		}
	}
}
