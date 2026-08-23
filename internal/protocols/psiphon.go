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
	"strings"
	"sync"

	"nexvpn/internal/config"
	"nexvpn/internal/sysproc"
)

// PsiphonConfig holds configuration for the Psiphon client
type PsiphonConfig struct {
	EgressRegion   string `json:"egress_region"`   // "US", "DE", "GB", "NL", "CA", "FR", "JP", "SG", "AT", "CH" or "" (fastest)
	SocksPort      int    `json:"socks_port"`      // default 10808
	HttpPort       int    `json:"http_port"`       // default 10809
	TunnelProtocol string `json:"tunnel_protocol"` // "SSH", "OSSH", "UNFRONTED-MEEK-OSSH", "FRONTED-MEEK-OSSH", "QUIC"
	DisableSplit   bool   `json:"disable_split"`   // if true, all traffic is tunneled
}

// PsiphonAdapter manages execution of the Psiphon circumvention tunnel
type PsiphonAdapter struct {
	mu          sync.RWMutex
	cmd         *exec.Cmd
	running     bool
	bytesRx     int64
	bytesTx     int64
	logger      LoggerFunc
	onExit      OnExitFunc
	statsCancel context.CancelFunc
	configPath  string
	dataDir     string
}

// NewPsiphonAdapter initializes a new Psiphon protocol adapter
func NewPsiphonAdapter() *PsiphonAdapter {
	return &PsiphonAdapter{}
}

// Name returns the protocol identifier
func (p *PsiphonAdapter) Name() config.Protocol {
	return config.ProtocolPsiphon
}

// SetLogger sets the log emitter
func (p *PsiphonAdapter) SetLogger(fn LoggerFunc) {
	p.mu.Lock()
	p.logger = fn
	p.mu.Unlock()
}

// SetOnExit sets the exit callback
func (p *PsiphonAdapter) SetOnExit(fn OnExitFunc) {
	p.mu.Lock()
	p.onExit = fn
	p.mu.Unlock()
}

func (p *PsiphonAdapter) log(level, msg string) {
	p.mu.RLock()
	logger := p.logger
	p.mu.RUnlock()
	if logger != nil {
		logger(level, msg, "psiphon")
	}
}

// Start launches the Psiphon tunnel engine
func (p *PsiphonAdapter) Start(ctx context.Context, configContent string) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("psiphon adapter is already running")
	}
	p.mu.Unlock()

	psiphonBin, err := findBinary("psiphon-tunnel-core")
	if err != nil {
		// Fallback to "psiphon" or "psi"
		psiphonBin, err = findBinary("psiphon")
		if err != nil {
			return fmt.Errorf("Psiphon core binary (psiphon-tunnel-core) not found in bin folder: %w", err)
		}
	}

	var cfg PsiphonConfig
	cfg.SocksPort = 10808
	cfg.HttpPort = 10809
	if configContent != "" && strings.HasPrefix(strings.TrimSpace(configContent), "{") {
		_ = json.Unmarshal([]byte(configContent), &cfg)
	}

	// Prepare data directory
	appData := ensureNexVPNTempDir()
	if appData == "" {
		appData = os.TempDir()
	}
	psiDir := filepath.Join(appData, "psiphon_data")
	_ = os.MkdirAll(psiDir, 0700)
	p.dataDir = psiDir

	// Build psiphon JSON config
	psiConfigObj := map[string]interface{}{
		"LocalSocksProxyPort": cfg.SocksPort,
		"LocalHttpProxyPort":  cfg.HttpPort,
		"ListenInterface":     "127.0.0.1",
		"DataRootDirectory":   psiDir,
		"TargetServerCount":   1,
	}
	if cfg.EgressRegion != "" && cfg.EgressRegion != "AUTO" {
		psiConfigObj["EgressRegion"] = strings.ToUpper(cfg.EgressRegion)
	}
	if cfg.TunnelProtocol != "" {
		psiConfigObj["TunnelProtocol"] = cfg.TunnelProtocol
	}

	cfgBytes, err := json.MarshalIndent(psiConfigObj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal psiphon config: %w", err)
	}

	cfgFile := filepath.Join(psiDir, "psiphon_active.json")
	if err := os.WriteFile(cfgFile, cfgBytes, 0600); err != nil {
		return fmt.Errorf("failed to write psiphon config file: %w", err)
	}
	p.configPath = cfgFile

	cmd := exec.CommandContext(ctx, psiphonBin, "-config", cfgFile, "-dataRootDirectory", psiDir, "-listenInterface", "127.0.0.1")
	cmd.SysProcAttr = sysproc.HideWindowAttr()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start psiphon: %w", err)
	}

	sysproc.AssignCmdToJobObject(cmd)

	p.mu.Lock()
	p.cmd = cmd
	p.running = true
	p.mu.Unlock()

	p.log("info", fmt.Sprintf("Psiphon Emergency Engine started (Region: %s, SOCKS: %d, HTTP: %d)", cfg.EgressRegion, cfg.SocksPort, cfg.HttpPort))

	// Stream logs
	go p.streamLogs(stdout, "stdout")
	go p.streamLogs(stderr, "stderr")

	// Supervise process exit
	go func() {
		waitErr := cmd.Wait()
		p.mu.Lock()
		p.running = false
		onExit := p.onExit
		p.mu.Unlock()

		if p.configPath != "" {
			_ = os.Remove(p.configPath)
		}

		if onExit != nil {
			onExit(waitErr)
		}
	}()

	return nil
}

func (p *PsiphonAdapter) streamLogs(r io.Reader, pipeName string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var notice struct {
				NoticeType string                 `json:"noticeType"`
				Data       map[string]interface{} `json:"data"`
				ShowUser   bool                   `json:"showUser"`
			}
			if err := json.Unmarshal([]byte(line), &notice); err == nil && notice.NoticeType != "" {
				switch notice.NoticeType {
				case "ListeningSocksProxyPort":
					p.log("info", fmt.Sprintf("Psiphon SOCKS5 proxy listening on port %v", notice.Data["port"]))
				case "ListeningHttpProxyPort":
					p.log("info", fmt.Sprintf("Psiphon HTTP proxy listening on port %v", notice.Data["port"]))
				case "Tunnels":
					p.log("info", fmt.Sprintf("Active Psiphon tunnels established: %v", notice.Data["count"]))
				case "ConnectingServer":
					p.log("info", fmt.Sprintf("Connecting to Psiphon node: %v (%v)", notice.Data["ipAddress"], notice.Data["region"]))
				case "ConnectedServer":
					p.log("info", fmt.Sprintf("Connected successfully to Psiphon server in %v", notice.Data["region"]))
				case "BytesTransferred":
					if rx, ok := notice.Data["bytesReceived"].(float64); ok {
						p.mu.Lock()
						if int64(rx) > p.bytesRx {
							p.bytesRx = int64(rx)
						} else {
							p.bytesRx += int64(rx)
						}
						p.mu.Unlock()
					}
					if tx, ok := notice.Data["bytesSent"].(float64); ok {
						p.mu.Lock()
						if int64(tx) > p.bytesTx {
							p.bytesTx = int64(tx)
						} else {
							p.bytesTx += int64(tx)
						}
						p.mu.Unlock()
					}
				default:
					if notice.ShowUser {
						p.log("info", fmt.Sprintf("[%s] %v", notice.NoticeType, notice.Data))
					}
				}
				continue
			}
		}

		p.log("info", line)
	}
}

// Stop terminates the Psiphon engine
func (p *PsiphonAdapter) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running || p.cmd == nil {
		return nil
	}

	p.running = false
	if p.cmd.Process != nil {
		sysproc.KillProcessTree(p.cmd.Process.Pid)
		_ = p.cmd.Process.Kill()
	}

	if p.configPath != "" {
		_ = os.Remove(p.configPath)
	}

	p.log("info", "Psiphon engine terminated gracefully")
	return nil
}

// GetStats returns current throughput
func (p *PsiphonAdapter) GetStats() (int64, int64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bytesRx, p.bytesTx, nil
}

// IsRunning reports if the process is active
func (p *PsiphonAdapter) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}
