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

// The bundled psiphon-tunnel-core is a bare console client: it ships no server
// entries of its own, so it MUST be told where to fetch them or it exits within
// a second having done nothing.
//
// These three values are the Psiphon Network's public "free network" bootstrap
// parameters, published in the upstream project's example configuration. The
// URL is deliberately base64-encoded because psiphon-tunnel-core base64-decodes
// the TransferURLs[].URL field ("to mitigate trivial binary executable string
// scanning"); a plain https:// URL there is a hard config error:
//
//	TransferURLs.DecodeAndValidate: failed to decode URL: illegal base64 data
const (
	// psiphonRemoteServerListURL is https://s3.amazonaws.com/psiphon/web/mjr4-p23r-puwl/server_list_compressed
	psiphonRemoteServerListURL = "aHR0cHM6Ly9zMy5hbWF6b25hd3MuY29tL3BzaXBob24vd2ViL21qcjQtcDIzci1wdXdsL3NlcnZlcl9saXN0X2NvbXByZXNzZWQ="

	// psiphonRemoteServerListPublicKey authenticates the downloaded list.
	psiphonRemoteServerListPublicKey = "MIICIDANBgkqhkiG9w0BAQEFAAOCAg0AMIICCAKCAgEAt7Ls+/39r+T6zNW7GiVpJfzq/xvL9SBH5rIFnk0RXYEYavax3WS6HOD35eTAqn8AniOwiH+DOkvgSKF2caqk/y1dfq47Pdymtwzp9ikpB1C5OfAysXzBiwVJlCdajBKvBZDerV1cMvRzCKvKwRmvDmHgphQQ7WfXIGbRbmmk6opMBh3roE42KcotLFtqp0RRwLtcBRNtCdsrVsjiI1Lqz/lH+T61sGjSjQ3CHMuZYSQJZo/KrvzgQXpkaCTdbObxHqb6/+i1qaVOfEsvjoiyzTxJADvSytVtcTjijhPEV6XskJVHE1Zgl+7rATr/pDQkw6DPCNBS1+Y6fy7GstZALQXwEDN/qhQI9kWkHijT8ns+i1vGg00Mk/6J75arLhqcodWsdeG/M/moWgqQAnlZAGVtJI1OgeF5fsPpXu4kctOfuZlGjVZXQNW34aOzm8r8S0eVZitPlbhcPiR4gT/aSMz/wd8lZlzZYsje/Jr8u/YtlwjjreZrGRmG8KMOzukV3lLmMppXFMvl4bxv6YFEmIuTsOhbLTwFgh7KYNjodLj/LsqRVfwz31PgWQFTEPICV7GCvgVlPRxnofqKSjgTWI4mxDhBpVcATvaoBl1L/6WLbFvBsoAUBItWwctO2xalKxF5szhGm8lccoc5MZr8kfE0uxMgsxz4er68iCID+rsCAQM="

	// psiphonRemoteServerListFilename is where the downloaded list is cached
	// under DataRootDirectory, so a later run can bootstrap with no network.
	psiphonRemoteServerListFilename = "remote_server_list"
)

// PsiphonAdapter manages execution of the Psiphon circumvention tunnel
type PsiphonAdapter struct {
	mu         sync.RWMutex
	cmd        *exec.Cmd
	running    bool
	bytesRx    int64
	bytesTx    int64
	logger     LoggerFunc
	onExit     OnExitFunc
	configPath string
	dataDir    string
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

// FindPsiphonServerList checks common locations for an embedded or cached server entry list file.
func FindPsiphonServerList() string {
	candidates := []string{
		"server_list.dat",
		"psiphon_servers.dat",
		"server_list.json",
	}
	for _, name := range candidates {
		if path, err := FindBinary(name); err == nil {
			if fi, sErr := os.Stat(path); sErr == nil && fi.Size() > 0 {
				return path
			}
		}
	}
	// Also check user data directories
	appData := ensureNexVPNTempDir()
	dataDirCandidates := []string{
		filepath.Join(appData, "psiphon_data", "server_list.dat"),
		filepath.Join(appData, "psiphon_servers.dat"),
	}
	for _, path := range dataDirCandidates {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return path
		}
	}
	return ""
}

// HasPsiphonServers checks if candidate servers exist via a server list file or populated datastore.
func HasPsiphonServers() bool {
	if FindPsiphonServerList() != "" {
		return true
	}
	// Check if local boltdb datastore has grown beyond initial empty size (32768 bytes)
	dbPath := filepath.Join(ensureNexVPNTempDir(), "psiphon_data", "ca.psiphon.PsiphonTunnel.tunnel-core", "datastore", "psiphon.boltdb")
	if fi, err := os.Stat(dbPath); err == nil && fi.Size() > 32768 {
		return true
	}
	return false
}

// buildPsiphonCoreConfig assembles the JSON object handed to the bundled
// psiphon-tunnel-core.
//
// Two things here are load-bearing and were both wrong before:
//
//  1. "ListenInterface" is deliberately NOT set. psiphon-tunnel-core reads that
//     field as a *network interface name* ("Ethernet", "Wi-Fi") or the literal
//     "any" — not as an address — and resolves it through
//     common.GetInterfaceIPAddresses. The old value of "127.0.0.1" is not an
//     interface, so the core aborted about a second after start with
//
//     error getting listener IP: common.GetInterfaceIPAddresses#38:
//     route ip+net: no such network interface
//
//     after emitting nothing but a NetworkID notice. Omitting the field makes the
//     core bind its local proxies to 127.0.0.1, which is what we want.
//
//  2. The RemoteServerList* fields are required. This build of the core ships no
//     server entries, so without a bootstrap source it has nothing to dial and
//     exits immediately. Note that TransferURLs[].URL is base64-encoded on
//     purpose (upstream obfuscates it against binary string scanning); passing a
//     plain https:// URL there is a hard config error.
func buildPsiphonCoreConfig(cfg PsiphonConfig, psiDir string) map[string]interface{} {
	obj := map[string]interface{}{
		"PropagationChannelId":               "FFFFFFFFFFFFFFFF",
		"SponsorId":                          "FFFFFFFFFFFFFFFF",
		"ClientVersion":                      "1",
		"LocalSocksProxyPort":                cfg.SocksPort,
		"LocalHttpProxyPort":                 cfg.HttpPort,
		"DataRootDirectory":                  filepath.ToSlash(psiDir),
		"RemoteServerListDownloadFilename":   psiphonRemoteServerListFilename,
		"RemoteServerListSignaturePublicKey": psiphonRemoteServerListPublicKey,
		"RemoteServerListURLs": []map[string]interface{}{
			{"URL": psiphonRemoteServerListURL, "OnlyAfterAttempts": 0},
		},
	}
	if cfg.EgressRegion != "" && cfg.EgressRegion != "AUTO" {
		obj["EgressRegion"] = strings.ToUpper(cfg.EgressRegion)
	}
	if cfg.TunnelProtocol != "" {
		obj["TunnelProtocol"] = cfg.TunnelProtocol
	}
	return obj
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
			return fmt.Errorf("psiphon core binary (psiphon-tunnel-core) not found in bin folder: %w", err)
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

	// Build the psiphon JSON config. See buildPsiphonCoreConfig for why
	// "ListenInterface" must never be set here.
	psiConfigObj := buildPsiphonCoreConfig(cfg, psiDir)

	cfgBytes, err := json.MarshalIndent(psiConfigObj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal psiphon config: %w", err)
	}

	cfgFile := filepath.Join(psiDir, "psiphon_active.json")
	if err := os.WriteFile(cfgFile, cfgBytes, 0600); err != nil {
		return fmt.Errorf("failed to write psiphon config file: %w", err)
	}
	p.configPath = cfgFile

	args := []string{"-config", cfgFile, "-dataRootDirectory", filepath.ToSlash(psiDir)}
	if serverList := FindPsiphonServerList(); serverList != "" {
		args = append(args, "-serverList", serverList)
	}

	cmd := exec.CommandContext(ctx, psiphonBin, args...)
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
			if waitErr != nil {
				p.log("error", fmt.Sprintf("Psiphon exited: %v (config: %s)", waitErr, cfgFile))
				onExit(waitErr)
			} else {
				p.log("warning", "Psiphon engine terminated (no active candidate servers or handshake aborted)")
				onExit(fmt.Errorf("NO_CANDIDATE_SERVERS: Psiphon core stopped unexpectedly (no servers available)"))
			}
		}
	}()

	return nil
}

func (p *PsiphonAdapter) streamLogs(r io.Reader, pipeName string) {
	defer func() {
		if rec := recover(); rec != nil {
			p.log("warn", fmt.Sprintf("Recovered in streamLogs: %v", rec))
		}
	}()

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
				if notice.Data == nil {
					continue
				}
				switch notice.NoticeType {
				case "ListeningSocksProxyPort":
					if port, ok := notice.Data["port"]; ok {
						p.log("info", fmt.Sprintf("Psiphon SOCKS5 proxy listening on port %v", port))
					}
				case "ListeningHttpProxyPort":
					if port, ok := notice.Data["port"]; ok {
						p.log("info", fmt.Sprintf("Psiphon HTTP proxy listening on port %v", port))
					}
				case "Tunnels":
					if count, ok := notice.Data["count"]; ok {
						p.log("info", fmt.Sprintf("Active Psiphon tunnels established: %v", count))
					}
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

		p.log("debug", fmt.Sprintf("[%s] %s", pipeName, line))
	}
}

// Stop terminates the Psiphon engine
func (p *PsiphonAdapter) Stop() error {
	p.mu.Lock()
	if !p.running || p.cmd == nil {
		p.mu.Unlock()
		return nil
	}

	cmd := p.cmd
	configPath := p.configPath
	p.running = false
	p.cmd = nil
	p.configPath = ""
	p.onExit = nil
	p.mu.Unlock()

	if cmd.Process != nil {
		_ = sysproc.KillProcessTree(cmd.Process.Pid)
		_ = cmd.Process.Kill()
	}
	if configPath != "" {
		_ = os.Remove(configPath)
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
