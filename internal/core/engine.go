// Package core implements the central VPN engine and orchestration subsystem for NexVPN.
//
// The Engine manages the entire lifecycle of VPN connections across diverse protocols
// (VLESS Reality, VMess, Trojan, Shadowsocks, Hysteria 2, TUIC, WireGuard / WARP / AmneziaWG, and OpenVPN).
// It coordinates:
//   - Adaptive multi-core dispatching (Sing-box, Xray, and Native OpenVPN with Wintun).
//   - Dual-layer routing (Virtual Wintun TUN Adapter + Local HTTP/SOCKS5 system proxy).
//   - Real-time traffic analytics, latency probes, and connection health monitoring.
//   - Automated failover and self-healing upon unexpected disconnection.
//   - Privacy shielding, DNS leak mitigation, and Kill Switch firewall enforcement.
package core

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/protocols"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

// CleanupStaleCores terminates any orphaned VPN core background processes (sing-box, xray, openvpn)
// that may have lingered from previous unexpected crashes to prevent port conflicts or device locking.
func CleanupStaleCores() {
	if runtime.GOOS == "windows" {
		for _, name := range []string{"sing-box.exe", "xray.exe", "openvpn.exe"} {
			cmd := exec.Command("taskkill", "/F", "/IM", name)
			cmd.SysProcAttr = sysproc.HideWindowAttr()
			_ = cmd.Run()
		}
		time.Sleep(100 * time.Millisecond)
	} else {
		for _, name := range []string{"sing-box", "xray", "openvpn"} {
			cmd := exec.Command("pkill", "-9", "-x", name)
			_ = cmd.Run()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func cleanupStaleCores() {
	CleanupStaleCores()
}

// ConnectionState represents the high-level operational phase of the VPN connection.
type ConnectionState string

const (
	// StateDisconnected indicates no VPN tunnel is currently active.
	StateDisconnected ConnectionState = "disconnected"
	// StateConnecting indicates the engine is actively initializing cores, probing ports, or negotiating handshakes.
	StateConnecting ConnectionState = "connecting"
	// StateConnected indicates the VPN tunnel and system routing are fully established and active.
	StateConnected ConnectionState = "connected"
	// StateDisconnecting indicates graceful termination and proxy cleanup are in progress.
	StateDisconnecting ConnectionState = "disconnecting"
	// StateError indicates a critical failure occurred during connection setup or runtime.
	StateError ConnectionState = "error"
)

// ConnectionStatus provides a real-time snapshot of current connection state and active profile metadata.
type ConnectionStatus struct {
	State       ConnectionState `json:"state"`
	ProfileID   string          `json:"profile_id,omitempty"`
	ProfileName string          `json:"profile_name,omitempty"`
	Protocol    config.Protocol `json:"protocol,omitempty"`
	ConnectedAt time.Time       `json:"connected_at,omitempty"`
	Server      string          `json:"server,omitempty"`
	ErrorMsg    string          `json:"error_msg,omitempty"`
	Duration    int64           `json:"duration,omitempty"` // Active connection duration in seconds
}

// TrafficStats captures real-time throughput metrics, bandwidth totals, and transfer rates.
type TrafficStats struct {
	BytesReceived   int64 `json:"bytes_received"`
	BytesSent       int64 `json:"bytes_sent"`
	SpeedDownload   int64 `json:"speed_download"` // Instantaneous download rate in bytes/sec
	SpeedUpload     int64 `json:"speed_upload"`   // Instantaneous upload rate in bytes/sec
	PacketsReceived int64 `json:"packets_received"`
	PacketsSent     int64 `json:"packets_sent"`
}

// LogEntry represents an individual formatted event record emitted by the engine or core adapters.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}

// Engine is the central orchestrator responsible for managing active VPN connections,
// system proxy configuration, traffic statistics sampling, and multi-core process supervision.
type Engine struct {
	mu            sync.RWMutex
	ctx           context.Context
	status        ConnectionStatus
	stats         TrafficStats
	logs          []LogEntry
	activeAdapter protocols.ProtocolAdapter
	cancelFn      context.CancelFunc
	activeProfile config.Profile
	lastSettings  config.Settings
	autoReconnect bool

	// Health Monitor for periodic heartbeat probing
	healthMonitor *HealthMonitor

	// Failover attempt counter to prevent infinite reconnect loops
	failoverAttempts int32

	statusCallbacks  []func(ConnectionStatus)
	logCallbacks     []func(LogEntry)
	statsCallbacks   []func(TrafficStats)
	healthCallbacks  []func(HealthStatus)
	failoverCallback func(currentProfileID string)
	sessionCallback  func(config.ConnectionSession)
}

// NewEngine creates and initializes a new instance of the Engine in the disconnected state.
func NewEngine() *Engine {
	return &Engine{
		status: ConnectionStatus{State: StateDisconnected},
		logs:   make([]LogEntry, 0, 500),
	}
}

// SetContext assigns the root context for child process management and cancellation trees.
func (e *Engine) SetContext(ctx context.Context) {
	e.ctx = ctx
}

// SetAutoReconnect enables or disables automated reconnect attempts on unexpected drops.
func (e *Engine) SetAutoReconnect(enable bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoReconnect = enable
}

// OnStatusChange registers an event listener invoked whenever the connection state changes.
func (e *Engine) OnStatusChange(cb func(ConnectionStatus)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusCallbacks = append(e.statusCallbacks, cb)
}

// OnLog registers an event listener invoked whenever a new log entry is recorded.
func (e *Engine) OnLog(cb func(LogEntry)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logCallbacks = append(e.logCallbacks, cb)
}

// OnStats registers an event listener invoked on every 1-second statistics sampling interval.
func (e *Engine) OnStats(cb func(TrafficStats)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statsCallbacks = append(e.statsCallbacks, cb)
}

// OnHealth registers an event listener invoked whenever health monitor latency probes update.
func (e *Engine) OnHealth(cb func(HealthStatus)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthCallbacks = append(e.healthCallbacks, cb)
}

// OnFailover registers a handler invoked when an active connection drops and auto-failover is triggered.
func (e *Engine) OnFailover(cb func(string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failoverCallback = cb
}

// OnSessionComplete registers a callback executed when a connection session ends to record history.
func (e *Engine) OnSessionComplete(cb func(config.ConnectionSession)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionCallback = cb
}

// Connect starts the VPN tunnel for the specified profile using the user's active settings.
//
// Workflow:
//  1. Transition state to StateConnecting and clean up any previous active sessions, cores, or system proxies.
//  2. Ensure Wintun virtual network adapter driver (wintun.dll) is available in system/bin path.
//  3. Invoke SmartCoreDispatcher to choose the optimal engine (Sing-box, Xray, or OpenVPN) and generate config JSON.
//  4. Launch the core process asynchronously in a dedicated cancellation context.
//  5. Verify readiness (probing local SOCKS5 port 10808 for proxy cores, or waiting for Wintun device initialization).
//  6. Configure system-wide routing (TUN virtual network device + dual HTTP/SOCKS5 system proxy).
//  7. Enforce Kill Switch leak prevention firewall rules if enabled.
//  8. Start background real-time bandwidth sampling and connection health heartbeat monitor.
//  9. Transition state to StateConnected and emit native OS toast notifications.
func (e *Engine) Connect(profile config.Profile, settings config.Settings) error {
	e.mu.Lock()
	currentState := e.status.State
	if currentState == StateConnecting {
		e.mu.Unlock()
		return fmt.Errorf("already connecting")
	}

	oldAdapter := e.activeAdapter
	oldCancel := e.cancelFn
	e.activeAdapter = nil
	e.cancelFn = nil

	e.status = ConnectionStatus{
		State:       StateConnecting,
		ProfileID:   profile.ID,
		ProfileName: profile.Name,
		Protocol:    profile.Protocol,
		Server:      profile.Server,
	}
	e.activeProfile = profile
	e.lastSettings = settings
	statusCopy := e.status
	e.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}
	if oldAdapter != nil {
		_ = oldAdapter.Stop()
	}
	cleanupStaleCores()
	waitForPortFree(10808, 1500*time.Millisecond)
	if currentState == StateConnected || currentState == StateDisconnecting {
		_ = sysproxy.ClearSystemProxy()
		time.Sleep(60 * time.Millisecond)
	}

	_ = GetKillSwitch().Disable()

	e.emitStatus(statusCopy)
	EnsureWintunDriver()

	dispatcher := NewSmartCoreDispatcher()
	decision, err := dispatcher.Decide(&profile, settings)
	if err != nil {
		e.setErrorStatus(err.Error())
		return err
	}

	adapter := decision.Adapter
	adapter.SetLogger(e.AddLog)

	adapter.SetOnExit(func(exitErr error) {
		e.AddLog("error", fmt.Sprintf("Core process crashed: %v", exitErr), "engine")
		_ = sysproxy.ClearSystemProxy()
		e.setErrorStatus(exitErr.Error())
	})

	configContent := decision.ConfigContent
	targetProtocol := decision.CoreProtocol

	e.AddLog("info", fmt.Sprintf("Smart Core: Selected %s (%s)", decision.CoreName, decision.Description), "engine")

	baseCtx := e.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	connCtx, cancel := context.WithCancel(baseCtx)

	e.mu.Lock()
	e.activeAdapter = adapter
	e.cancelFn = cancel
	e.mu.Unlock()

	go func() {
		err := adapter.Start(connCtx, configContent)
		if err != nil {
			cancel()
			e.AddLog("error", fmt.Sprintf("Connection failed on %s: %v", decision.CoreName, err), "engine")
			_ = sysproxy.ClearSystemProxy()
			e.setErrorStatus(err.Error())
			return
		}

		if targetProtocol == config.ProtocolOpenVPN {
			// OpenVPN is a Layer-3 TUN adapter tunnel and does not open a local SOCKS port (10808).
			// Allow time for the Wintun adapter driver to initialize
			time.Sleep(1200 * time.Millisecond)
		} else {
			timeout := 150 * time.Second
			if targetProtocol == config.ProtocolPsiphon {
				timeout = 240 * time.Second
			}
			if !probeProxy(10808, timeout) {
				cancel()
				errMsg := "proxy port 10808 did not become available — core may have failed to start"
				e.AddLog("error", errMsg, "engine")
				_ = sysproxy.ClearSystemProxy()
				e.setErrorStatus(errMsg)
				return
			}
		}

		mode := settings.ProxyMode
		if mode == "" {
			mode = "tun"
		}
		if targetProtocol == config.ProtocolOpenVPN {
			_ = sysproxy.ClearSystemProxy()
			e.AddLog("info", "OpenVPN Direct TUN Adapter Active (Full-System Routing with Wintun)", "engine")
		} else {
			_ = sysproxy.SetFullSystemProxy(10809, 10808)
			if mode == "tun" && targetProtocol == config.ProtocolSingBox {
				e.AddLog("info", "Dual-Routing Active: TUN Adapter (NexVPN-TUN) + System Proxy (127.0.0.1:10809/10808)", "engine")
			} else {
				e.AddLog("info", "System Proxy enabled on 127.0.0.1:10809 (HTTP) and 127.0.0.1:10808 (SOCKS5)", "engine")
			}
		}

		if settings.KillSwitchEnabled {
			_ = GetKillSwitch().Enable(profile.Server)
			e.AddLog("info", "Kill Switch enabled (Leak prevention active)", "engine")
		}

		atomic.StoreInt32(&e.failoverAttempts, 0)

		// Start Smart Health Monitor for proxy protocols
		var hm *HealthMonitor
		if targetProtocol != config.ProtocolOpenVPN {
			hm = NewHealthMonitor("http://cp.cloudflare.com/generate_204", 10808)
			hm.SetCallback(func(hs HealthStatus) {
				e.emitHealth(hs)
			})
			hm.SetUnhealthyCallback(func(fails int) {
				e.AddLog("warning", fmt.Sprintf("Connection unstable: %d consecutive health check failures", fails), "health")
			})
			go hm.Start(connCtx)
		}

		e.mu.Lock()
		e.healthMonitor = hm
		e.status.State = StateConnected
		e.status.ConnectedAt = time.Now()
		st := e.status
		e.mu.Unlock()

		e.emitStatus(st)
		e.AddLog("info", fmt.Sprintf("Successfully connected to %s", profile.Name), "engine")

		// Windows Native Toast Notification
		if settings.NotificationsEnabled {
			go ShowWindowsNotification("NexVPN Connected", fmt.Sprintf("Connected to %s (%s)", profile.Name, profile.Protocol))
		}

		e.startStatsLoop(connCtx, adapter)
	}()

	return nil
}

// waitForPortFree waits until a TCP port is released and available for listening
func waitForPortFree(port int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = ln.Close()
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// probeProxy repeatedly checks whether the local SOCKS5 or HTTP proxy port is accepting TCP sockets.
func probeProxy(port int, timeout time.Duration) bool {
	if port <= 0 {
		port = 10808
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Probe primary target port on 127.0.0.1
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 350*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}

		// Also check HTTP port 10809 if standard SOCKS port 10808 is targeted
		if port == 10808 {
			connHttp, errHttp := net.DialTimeout("tcp", "127.0.0.1:10809", 350*time.Millisecond)
			if errHttp == nil {
				connHttp.Close()
				return true
			}
		}

		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// Disconnect gracefully stops the active VPN connection and cleans up all system proxy / firewall modifications.
func (e *Engine) Disconnect() error {
	e.mu.Lock()
	if e.status.State == StateDisconnected {
		e.mu.Unlock()
		return nil
	}

	e.status.State = StateDisconnecting
	st := e.status
	adapter := e.activeAdapter
	cancel := e.cancelFn
	activeP := e.activeProfile
	connectedAt := e.status.ConnectedAt
	statsSnapshot := e.stats
	sessionCb := e.sessionCallback
	notifyEnabled := e.lastSettings.NotificationsEnabled

	if adapter != nil {
		adapter.SetOnExit(nil)
	}
	e.activeAdapter = nil
	e.cancelFn = nil
	e.healthMonitor = nil
	e.mu.Unlock()

	_ = sysproxy.ClearSystemProxy()
	_ = GetKillSwitch().Disable()

	e.emitStatus(st)
	e.AddLog("info", "Disconnecting...", "engine")

	if cancel != nil {
		cancel()
	}

	if adapter != nil {
		_ = adapter.Stop()
	}

	// Record Session History
	if sessionCb != nil && activeP.ID != "" && !connectedAt.IsZero() {
		dur := int64(time.Since(connectedAt).Seconds())
		session := config.ConnectionSession{
			ID:             fmt.Sprintf("sess_%d", time.Now().UnixNano()),
			ProfileID:      activeP.ID,
			ProfileName:    activeP.Name,
			Protocol:       activeP.Protocol,
			Server:         activeP.Server,
			ConnectedAt:    connectedAt,
			DisconnectedAt: time.Now(),
			DurationSec:    dur,
			BytesReceived:  statsSnapshot.BytesReceived,
			BytesSent:      statsSnapshot.BytesSent,
			Reason:         "manual",
		}
		go sessionCb(session)
	}

	e.mu.Lock()
	e.status = ConnectionStatus{State: StateDisconnected}
	e.activeAdapter = nil
	e.cancelFn = nil
	e.stats = TrafficStats{}
	finalStatus := e.status
	e.mu.Unlock()

	e.emitStatus(finalStatus)
	e.AddLog("info", "Disconnected. System proxy cleaned.", "engine")

	if notifyEnabled {
		go ShowWindowsNotification("NexVPN Disconnected", "Disconnected from server.")
	}

	return nil
}

// startStatsLoop periodically samples network bandwidth, adapter throughput, and connection duration every second.
func (e *Engine) startStatsLoop(ctx context.Context, adapter protocols.ProtocolAdapter) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastRx, lastTx int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rx, tx, err := adapter.GetStats()
			if (rx == 0 && tx == 0) || err != nil {
				if tunRx, tunTx, found := QueryTunnelAdapterBytes(); found {
					rx = tunRx
					tx = tunTx
				}
			}

			e.mu.Lock()
			// Fix: only calculate duration if actually connected and ConnectedAt is non-zero
			var duration int64
			if e.status.State == StateConnected && !e.status.ConnectedAt.IsZero() {
				duration = int64(time.Since(e.status.ConnectedAt).Seconds())
			}
			e.status.Duration = duration

			speedDown := rx - lastRx
			speedUp := tx - lastTx
			if lastRx == 0 || speedDown < 0 {
				speedDown = 0
			}
			if lastTx == 0 || speedUp < 0 {
				speedUp = 0
			}

			lastRx = rx
			lastTx = tx

			e.stats = TrafficStats{
				BytesReceived: rx,
				BytesSent:     tx,
				SpeedDownload: speedDown,
				SpeedUpload:   speedUp,
			}

			statsCopy := e.stats
			stCopy := e.status
			e.mu.Unlock()

			e.emitStats(statsCopy)
			e.emitStatus(stCopy)
		}
	}
}

func (e *Engine) setErrorStatus(msg string) {
	_ = sysproxy.ClearSystemProxy()

	e.mu.Lock()
	previousState := e.status.State
	if previousState == StateDisconnecting || previousState == StateDisconnected {
		e.mu.Unlock()
		return
	}

	e.status.State = StateError
	e.status.ErrorMsg = msg
	st := e.status
	e.activeAdapter = nil
	autoReconn := e.autoReconnect
	activeP := e.activeProfile
	lastS := e.lastSettings
	connectedAt := e.status.ConnectedAt
	statsSnapshot := e.stats
	sessionCb := e.sessionCallback
	e.mu.Unlock()

	e.emitStatus(st)

	// Record Session History on error
	if sessionCb != nil && activeP.ID != "" && !connectedAt.IsZero() {
		dur := int64(time.Since(connectedAt).Seconds())
		session := config.ConnectionSession{
			ID:             fmt.Sprintf("sess_%d", time.Now().UnixNano()),
			ProfileID:      activeP.ID,
			ProfileName:    activeP.Name,
			Protocol:       activeP.Protocol,
			Server:         activeP.Server,
			ConnectedAt:    connectedAt,
			DisconnectedAt: time.Now(),
			DurationSec:    dur,
			BytesReceived:  statsSnapshot.BytesReceived,
			BytesSent:      statsSnapshot.BytesSent,
			Reason:         "error",
		}
		go sessionCb(session)
	}

	const defaultMaxFailoverAttempts = 3
	maxFailoverAttempts := int32(defaultMaxFailoverAttempts)
	// Use user-configured reconnect attempts if available
	if lastS.ReconnectAttempts > 0 {
		maxFailoverAttempts = int32(lastS.ReconnectAttempts)
	}

	isAutoFailoverEnabled := lastS.AutoFailover || lastS.AutoFailoverEnabled
	// Only trigger auto-failover if the session was previously established (StateConnected)
	// and then dropped unexpectedly mid-connection, to avoid jumping servers on misconfiguration
	if isAutoFailoverEnabled && previousState == StateConnected && e.failoverCallback != nil && activeP.ID != "" {
		attempts := atomic.AddInt32(&e.failoverAttempts, 1)
		if attempts <= maxFailoverAttempts {
			e.AddLog("warning", fmt.Sprintf("[Auto-Failover] Active connection dropped (attempt %d/%d). Switching to backup server...", attempts, maxFailoverAttempts), "engine")
			if lastS.NotificationsEnabled {
				go ShowWindowsNotification("NexVPN Auto-Failover", "Connection dropped. Switching to best backup server...")
			}
			go func() {
				time.Sleep(1 * time.Second)
				e.failoverCallback(activeP.ID)
			}()
		} else {
			e.AddLog("error", fmt.Sprintf("[Auto-Failover] Max failover attempts (%d) reached. Stopping.", maxFailoverAttempts), "engine")
			atomic.StoreInt32(&e.failoverAttempts, 0)
		}
	} else if autoReconn && previousState == StateConnected && activeP.ID != "" {
		e.AddLog("warning", "Connection dropped. Auto-reconnecting in 3 seconds...", "engine")
		go func() {
			time.Sleep(3 * time.Second)
			_ = e.Connect(activeP, lastS)
		}()
	} else {
		atomic.StoreInt32(&e.failoverAttempts, 0)
	}
}

func (e *Engine) emitStatus(st ConnectionStatus) {
	e.mu.RLock()
	cbs := make([]func(ConnectionStatus), len(e.statusCallbacks))
	copy(cbs, e.statusCallbacks)
	e.mu.RUnlock()

	for _, cb := range cbs {
		cb(st)
	}
}

func (e *Engine) emitStats(st TrafficStats) {
	e.mu.RLock()
	cbs := make([]func(TrafficStats), len(e.statsCallbacks))
	copy(cbs, e.statsCallbacks)
	e.mu.RUnlock()

	for _, cb := range cbs {
		cb(st)
	}
}

func (e *Engine) emitHealth(hs HealthStatus) {
	e.mu.RLock()
	cbs := make([]func(HealthStatus), len(e.healthCallbacks))
	copy(cbs, e.healthCallbacks)
	e.mu.RUnlock()

	for _, cb := range cbs {
		cb(hs)
	}
}

// AddLog logs an event with the specified severity level, message, and subsystem origin.
// If Privacy Shield is enabled in settings, sensitive tokens, UUIDs, and credentials are automatically redacted.
func (e *Engine) AddLog(level string, message string, source string) {
	// Privacy Shield: Mask sensitive tokens and UUIDs if enabled
	e.mu.RLock()
	shieldEnabled := e.lastSettings.PrivacyShieldEnabled
	e.mu.RUnlock()

	if shieldEnabled {
		message = RedactSensitiveInfo(message)
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	e.mu.Lock()
	if len(e.logs) >= 1000 {
		e.logs = e.logs[1:]
	}
	e.logs = append(e.logs, entry)
	cbs := make([]func(LogEntry), len(e.logCallbacks))
	copy(cbs, e.logCallbacks)
	e.mu.Unlock()

	for _, cb := range cbs {
		cb(entry)
	}
}

// GetStatus returns a thread-safe copy of the current connection status.
func (e *Engine) GetStatus() ConnectionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// GetStats returns a thread-safe copy of the real-time traffic statistics.
func (e *Engine) GetStats() TrafficStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetLogs returns a snapshot copy of the in-memory circular log buffer.
func (e *Engine) GetLogs() []LogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	res := make([]LogEntry, len(e.logs))
	copy(res, e.logs)
	return res
}

// ClearLogs empties the in-memory circular log buffer.
func (e *Engine) ClearLogs() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = make([]LogEntry, 0, 500)
}
