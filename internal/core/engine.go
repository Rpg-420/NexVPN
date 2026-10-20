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
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/parser"
	"nexvpn/internal/protocols"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

// CleanupStaleCores terminates any orphaned VPN core background processes
// (sing-box, xray, openvpn, aether, psiphon) that may have lingered from previous
// unexpected crashes to prevent port conflicts or device locking.
//
// Every PID is verified against its executable image before it is killed. The
// recorded PID alone is not evidence: the pid file survives a crash, Windows
// reuses PIDs, and this process runs elevated — so a stale entry can name any
// process on the machine, and acting on it would terminate an unrelated
// application. An image name that cannot be read is a refusal, not a match.
func CleanupStaleCores() {
	pids := sysproc.GetRegisteredPIDs()
	for _, pid := range pids {
		if pid <= 0 || pid == os.Getpid() {
			continue
		}
		if runtime.GOOS == "windows" {
			image := sysproc.ProcessImageName(pid)
			if !isOwnCoreImage(image) {
				diary.Log("connect", "stale-pid-skipped",
					"pid", strconv.Itoa(pid),
					"image", image)
				continue
			}
			diary.Log("connect", "stale-pid-killed", "pid", strconv.Itoa(pid), "image", image)
		}
		sysproc.KillProcessTree(pid)
	}
	sysproc.ClearPIDs()
}

// SweepIdleCores terminates core processes this application started that no
// session owns any more, and reports how many were examined.
//
// The state guard is the whole design. CleanupStaleCores kills every registered
// PID and then clears the list, so running it on a timer would take the live core
// down with the tunnel. This is the variant that is safe to schedule: it acts only
// when nothing is connected or being established, which is exactly when a
// surviving process is an orphan rather than a session.
//
// Why it is worth a timer at all: a core that outlives its Disconnect keeps
// 10808 and the Wintun pool, and the next connect then spends its whole fallback
// ladder on a port conflict that reads like an unrelated bug. Before this, the
// only sweep happened at startup, so the orphan was cleared a restart later —
// after the user had already been told their server is dead.
func (e *Engine) SweepIdleCores() int {
	e.mu.RLock()
	state := e.status.State
	e.mu.RUnlock()

	if state == StateConnected || state == StateConnecting || state == StateDisconnecting {
		return 0
	}

	pids := sysproc.GetRegisteredPIDs()
	if len(pids) == 0 {
		return 0
	}

	diary.Log("maint", "idle-core-sweep", "registered", strconv.Itoa(len(pids)))
	CleanupStaleCores()
	return len(pids)
}

// ConnectionState represents the high-level operational phase of the VPN connection.
type ConnectionState string

const (
	// maxLogEntries is how many log lines the UI can see.
	maxLogEntries = 1000
	// logTrimBatch is how many lines are dropped at once when the buffer is
	// full. See AddLog: trimming one line at a time made every log write O(n).
	logTrimBatch = 250
)

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
// defaultMaxFailoverAttempts caps automatic server switching (both on abrupt
// disconnect and on health-monitor-detected outages) unless the user configured
// a different ReconnectAttempts value.
const defaultMaxFailoverAttempts = 3

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

	// lastCoreExitErr is the error the active core died with, kept so the
	// verification step can tell "the server is unreachable" apart from "the core
	// could not set up its TUN adapter".
	//
	// Those two look identical from the probe's point of view — neither passes
	// traffic — but only the first is worth failing over to another server. A TUN
	// adapter fault is local and reproduces identically on every server, so
	// failing over just walks the whole list while burning the attempt budget.
	lastCoreExitErr error

	// sessionDataAlertBytes is the per-session traffic threshold from the
	// profile, or 0 for none. It is an alert, not a hard cut: the cores expose
	// no portable way to enforce a byte ceiling, and silently killing a
	// connection mid-transfer would be a worse surprise than the data it saves.
	sessionDataAlertBytes int64
	dataAlertFired        bool

	operationID      uint64
	statusCallbacks  []func(ConnectionStatus)
	logCallbacks     []func(LogEntry)
	statsCallbacks   []func(TrafficStats)
	healthCallbacks  []func(HealthStatus)
	failoverCallback func(currentProfileID string)
	sessionCallback  func(config.ConnectionSession)

	// serverHealthCallback reports the verdict of a connect attempt on the server
	// itself: "this endpoint proved unreachable" or "this endpoint carried
	// traffic". The app turns it into a per-profile cooldown so that the next
	// failover, the next "fastest server" run and the next automatic retry do not
	// walk straight back onto a host that refused a connection seconds ago.
	//
	// It is a callback rather than a store reference because the engine stays
	// persistence-free, like every other part of this package.
	serverHealthCallback func(profileID string, healthy bool, reason string)
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

// OnServerHealth registers a callback that reports whether a profile's server
// proved reachable during a connect attempt. See the serverHealthCallback field.
func (e *Engine) OnServerHealth(cb func(profileID string, healthy bool, reason string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.serverHealthCallback = cb
}

// reportServerHealth publishes the verdict without holding the engine lock while
// the listener runs: the app's handler writes to the store, and a store write
// must never be able to block a status update or another log line.
func (e *Engine) reportServerHealth(profileID string, healthy bool, reason string) {
	if profileID == "" {
		return
	}

	e.mu.RLock()
	cb := e.serverHealthCallback
	e.mu.RUnlock()

	if cb != nil {
		cb(profileID, healthy, reason)
	}
}

// coreExitCleanup is what a dead core leaves behind that has to be undone. The
// two connect paths differ only here, which is why they share one handler.
type coreExitCleanup int

const (
	// exitClearProxy: the primary path may have published a system proxy pointing
	// at the port the core just stopped listening on.
	exitClearProxy coreExitCleanup = iota
	// exitFlushNetwork: the System-Proxy fallback is reached only after an
	// adapter fault, so the resolver and route state it may have disturbed are
	// reset as well.
	exitFlushNetwork
)

// handleCoreExit is the single reaction to a core process dying on its own.
//
// The primary adapter and the fallback adapter used to carry two hand-copied
// versions of this body, and one of them logged the same "Core process crashed"
// line twice per death — visible as a back-to-back pair in the 2026-10-12 log.
// One implementation means one line, and the two paths can no longer drift.
func (e *Engine) handleCoreExit(operationID uint64, exitErr error, cleanup coreExitCleanup) {
	if !e.isCurrentOperation(operationID) {
		return
	}

	// Recorded before anything else: a core that dies on its own is the single
	// most common reason a connect attempt stalls, and without this line the
	// diary just stops mid-attempt with no explanation.
	diary.LogErr("connect", "core-exited", exitErr)

	e.mu.Lock()
	e.lastCoreExitErr = exitErr
	e.mu.Unlock()

	// The core's own error is the diagnosis, so it is the message — but a nil
	// exit error must not print "crashed: %!v(<nil>)".
	errMsg := "Core process exited unexpectedly"
	if exitErr != nil {
		errMsg = fmt.Sprintf("Core process crashed: %v", exitErr)
	}
	e.AddLog("error", errMsg, "engine")

	switch cleanup {
	case exitClearProxy:
		if err := sysproxy.ClearSystemProxy(); err != nil {
			e.AddLog("error", fmt.Sprintf("Failed to clear system proxy after crash: %v", err), "engine")
		}
	case exitFlushNetwork:
		_ = sysproxy.FlushDNSAndResetNetwork()
	}

	e.setErrorStatus(errMsg)
}

// phaseTimer records how long each step of a connect attempt takes.
//
// Real diaries show 9-33 seconds between `connect/begin` and
// `connect/engine-select` — a variable, unexplained gap that dominates how
// sluggish the app feels — and nothing in the file said which step was
// responsible. Guessing at it is not possible from the outside, so each step
// reports its own duration.
//
// The marks are written as `connect/phase` lines with a `step` and `took` field.
type phaseTimer struct {
	last time.Time
}

func (p *phaseTimer) mark(step string) {
	now := time.Now()
	if !p.last.IsZero() {
		diary.Log("connect", "phase",
			"step", step,
			"took", now.Sub(p.last).Round(time.Millisecond).String())
	}
	p.last = now
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
// prepareConnectionMode runs everything between "kill switch is off" and "core
// is selected": announce the connecting state, clear our own TUN adapter (so a
// ghost cannot satisfy the readiness probe), enforce the TUN elevation guard,
// and make sure the Wintun driver is present. It returns the (possibly
// downgraded) settings the dispatcher must use.
//
// Extracted from Connect to keep the connection flow readable; it mutates no
// engine state beyond lastSettings/lastCoreExitErr bookkeeping.
func (e *Engine) prepareConnectionMode(profile config.Profile, settings config.Settings, statusCopy ConnectionStatus, phase *phaseTimer) config.Settings {
	e.emitStatus(statusCopy)
	// Clear NexVPN's own TUN adapter before a TUN core is spawned, and pick an
	// adapter name that is not occupied by a ghost. See PrepareTUNForConnect: a
	// leftover or ghost adapter satisfies the core's readiness probe immediately
	// and makes a dying core look connected.
	settings = PrepareTUNForConnect(settings)
	phase.mark("tun-prepare")

	// A TUN core has to create and configure a Wintun adapter, and Windows grants
	// that only to an elevated process. This guard is placed *before* dispatch
	// deliberately: once a doomed TUN config is built the core still starts, the
	// adapter still appears (so the readiness probe reports `tun-ready`), and only
	// then does the core die with
	//
	//	configure tun interface: Access is denied.
	//
	// which the engine cannot tell apart from a dead server — so it spends its
	// entire auto-failover budget switching between perfectly healthy servers.
	// Refusing to build that config in the first place removes the whole cascade.
	//
	// The shipped manifest requests Administrator, so this is a safety net for a
	// process that somehow started unelevated. Downgrading to System Proxy keeps
	// the user connected, which is strictly better than a guaranteed failure.
	e.mu.Lock()
	e.lastCoreExitErr = nil
	e.mu.Unlock()
	if tunModeRequested(settings) && runtime.GOOS == "windows" && !sysproc.IsAdmin() {
		settings.ProxyMode = "system_proxy"
		// Keep lastSettings in step too, so an auto-reconnect does not rebuild the
		// same doomed TUN config and repeat the warning.
		e.mu.Lock()
		e.lastSettings = settings
		e.mu.Unlock()
		diary.Log("connect", "tun-needs-admin", "fallback", "system_proxy", "profile", profile.Name)
		e.AddLog("warning", tunNeedsAdminMessage, "engine")
	}
	phase.mark("tun-admin-check")
	EnsureWintunDriver()
	phase.mark("wintun-driver")
	return settings
}

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
	e.sessionDataAlertBytes = int64(profile.SessionDataCapMB) * 1024 * 1024
	e.dataAlertFired = false
	e.operationID++
	operationID := e.operationID
	statusCopy := e.status
	e.mu.Unlock()

	// Record that a connection is in flight before any destructive step runs.
	// If the process dies from here on, this marker is the only thing that tells
	// the next launch that the machine may be holding an orphaned core, a stale
	// TUN adapter, a system proxy pointing at a dead port and a half-applied
	// kill switch. See AutoRepairFromJournal.
	WriteConnectJournal(ConnectJournal{
		Stage:     "connecting",
		ProfileID: profile.ID,
		Profile:   profile.Name,
		Protocol:  string(profile.Protocol),
		Server:    profile.Server,
		Mode:      settings.ProxyMode,
		PID:       os.Getpid(),
	})

	diary.Log("connect", "begin",
		"profile", profile.Name,
		"protocol", string(profile.Protocol),
		"server", profile.Server,
		"mode", settings.ProxyMode,
		"from_state", string(currentState),
	)
	phase := &phaseTimer{} // started here: the gap that matters begins at `begin`

	if oldCancel != nil {
		oldCancel()
	}
	if oldAdapter != nil {
		_ = oldAdapter.Stop()
	}
	phase.mark("stop-previous-core")
	CleanupStaleCores()
	phase.mark("cleanup-stale-cores")
	// 10808 is hard-coded as the SOCKS/mixed inbound throughout the app —
	// dispatcher, health monitor, sysproxy and the Telegram SOCKS link all point
	// at it — so it cannot be moved per-connection without changing all of them.
	// That makes proving it is free a precondition, not a courtesy.
	if !releaseProxyPort(10808, 4*time.Second) {
		err := fmt.Errorf(
			"local proxy port 10808 is still in use (%s). Close any other VPN client or restart NexVPN, then try again",
			sysproc.DescribeProcessHoldingPort(10808))
		diary.LogErr("connect", "proxy-port-busy", err)
		e.AddLog("error", err.Error(), "engine")
		e.setErrorStatus(err.Error())
		return err
	}
	phase.mark("release-proxy-port")
	if currentState == StateConnected || currentState == StateDisconnecting {
		if err := sysproxy.ClearSystemProxy(); err != nil {
			e.AddLog("error", fmt.Sprintf("Failed to clear system proxy: %v", err), "engine")
		}
		time.Sleep(60 * time.Millisecond)
	}
	phase.mark("clear-system-proxy")

	if err := GetKillSwitch().Disable(); err != nil {
		e.AddLog("error", fmt.Sprintf("Failed to disable Kill Switch: %v", err), "engine")
		// Continue with disconnect process but log the error
	}
	phase.mark("kill-switch-off")

	settings = e.prepareConnectionMode(profile, settings, statusCopy, phase)

	dispatcher := NewSmartCoreDispatcher()
	decision, err := dispatcher.Decide(&profile, settings)
	phase.mark("select-core")
	if err != nil {
		diary.LogErr("connect", "engine-select-failed", err)
		e.setErrorStatus(err.Error())
		return err
	}
	diary.Log("connect", "engine-select", "core", decision.CoreName, "protocol", string(decision.CoreProtocol))

	adapter := decision.Adapter
	adapter.SetLogger(e.AddLog)
	adapter.SetOnExit(func(exitErr error) {
		e.handleCoreExit(operationID, exitErr, exitClearProxy)
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

	diary.Log("connect", "spawn", "core", decision.CoreName, "mode", settings.ProxyMode)

	go func() {
		isCurrent := func() bool {
			e.mu.RLock()
			defer e.mu.RUnlock()
			return e.operationID == operationID
		}

		// A core that dies on its own announces itself through SetOnExit, which
		// clears the system proxy and moves the engine to StateError. This
		// goroutine keeps running regardless: `operationID` never changes, so
		// `isCurrent()` still returns true and every later stage happily proceeds.
		//
		// That is exactly how Psiphon reported "connected" with no tunnel at all —
		// the core exited with NO_CANDIDATE_SERVERS, the pipeline then re-applied
		// the system proxy pointing at a port nobody was listening on (which takes
		// the machine offline), and finally overwrote StateError with
		// StateConnected. So every stage after the spawn has to ask "is the core
		// still alive?", not merely "is this still my operation?".
		aborted := false
		abortIfCoreDead := func(stage string) bool {
			if aborted {
				return true
			}
			e.mu.RLock()
			state := e.status.State
			exitErr := e.lastCoreExitErr
			e.mu.RUnlock()

			stale := !isCurrent()
			if !stale && state == StateConnecting && exitErr == nil {
				return false
			}
			aborted = true

			exitMsg := ""
			if exitErr != nil {
				exitMsg = exitErr.Error()
			}
			diary.Log("connect", "abort-core-dead",
				"stage", stage,
				"state", string(state),
				"stale", strconv.FormatBool(stale),
				"exit", exitMsg)

			// Undo the half-applied session. A system proxy aimed at a dead local
			// port is worse than no proxy at all: every request the user makes
			// fails, which is the "it says connected but my internet is gone"
			// report. SetOnExit clears it too, but the crash may have landed
			// between two stages here and the pipeline re-applied it afterwards.
			_ = sysproxy.ClearSystemProxy()
			_ = adapter.Stop()
			cancel()
			return true
		}

		if targetProtocol == config.ProtocolOpenVPN {
			ReleaseStaleTUNAdapter()
			time.Sleep(600 * time.Millisecond)
		}
		err := adapter.Start(connCtx, configContent)
		// fallbackNote records why the TUN -> System Proxy fallback did not end up
		// running, so the failure the user is shown names the attempt that was
		// made instead of only the original adapter error.
		var fallbackNote string
		if err != nil {
			if abortIfCoreDead("post-start") {
				return
			}
			// Auto-fallback: if TUN creation failed on Sing-Box/Xray, retry in System Proxy mode
			if settings.ProxyMode == "tun" || settings.ProxyMode == "" {
				// Only proxy-based cores can fall back to System Proxy mode. OpenVPN
				// always requires a TUN adapter, so retrying it in that mode simply
				// repeats the same Wintun failure.
				//
				// A config-decode failure is skipped too: the fallback rebuilds the
				// same config with ProxyMode=system_proxy and the core dies at the
				// same line, which only adds a few seconds and a second, differently
				// worded error. Retrying helps for adapter faults, not for a config
				// the core cannot parse.
				if targetProtocol != config.ProtocolOpenVPN && !isConfigDecodeError(err) {
					e.AddLog("warning", fmt.Sprintf("TUN Adapter startup failed (%v). Auto-falling back to System Proxy mode...", err), "engine")
					diary.Log("connect", "fallback", "from", "tun", "to", "system_proxy", "reason", err.Error())

					// Stop the first core and prove 10808 is free before the
					// fallback core is built.
					//
					// A TUN adapter fault is reported by the core *process*, not
					// by Start's own bookkeeping, so the core is frequently still
					// running and holding the listener when Start returns an
					// error. Starting a second core on top of it produced
					//
					//	bind: Only one usage of each socket address
					//
					// which reads like an unrelated bug and leaves the user with
					// two dead cores. Stopping first, then binding the port
					// ourselves to prove it is free, removes the overlap.
					_ = adapter.Stop()
					if !releaseProxyPort(10808, 5*time.Second) {
						// ABORT the fallback instead of carrying on into it.
						//
						// This is the branch that produced
						// `bind: Only one usage of each socket address`. The port
						// is still held by the core that just "stopped", so the
						// fallback core would fail to bind, die, and take the
						// whole connection down with a second, unrelated-looking
						// error. Bailing out here leaves the user with the one
						// real error (the TUN failure above) instead of a
						// misleading port conflict, and the core-dead guard
						// below still runs.
						owner := sysproc.DescribeProcessHoldingPort(10808)
						diary.Log("connect", "fallback-port-busy", "port", "10808", "owner", owner)
						fallbackNote = fmt.Sprintf(
							"System Proxy fallback aborted: 127.0.0.1:10808 is still held by %s after stopping the core", owner)
						e.AddLog("error", fmt.Sprintf(
							"%s. Starting a second core would fail with 'bind: Only one usage of each socket address'.",
							fallbackNote), "engine")
					} else {
						// Delete the adapter rather than just re-enabling it: the
						// failure mode that actually blocks a reconnect is a ghost
						// Wintun adapter, which `netsh ... admin=enable` cannot see
						// and therefore cannot repair. Leaving it in place would
						// break the *next* TUN attempt too.
						//
						// Gated on a scan, because the removal costs a PowerShell
						// launch plus a 1.5 s settle and used to run on every
						// fallback regardless — which is where the ~41 s gap
						// between the TUN failure and the fallback core starting
						// came from. When the scan sees nothing of ours there is
						// nothing to remove.
						if staleAdaptersPresent(protocols.ScanWintunGhosts()) {
							RemoveStaleTUNAdapter()
							// Give Windows a moment to actually release the adapter
							// before the fallback core touches the network stack.
							time.Sleep(1500 * time.Millisecond)
						} else {
							diary.Log("connect", "fallback-removal-skipped")
						}
						fallbackSettings := settings
						fallbackSettings.ProxyMode = "system_proxy"
						fbDecision, fbErr := dispatcher.Decide(&profile, fallbackSettings)
						if fbErr != nil {
							// The fallback was announced and then never happened.
							// Silence here is what made the 2026-10-11 session read
							// as "Auto-falling back to System Proxy mode…" followed
							// by 37 dead seconds and the original TUN error: a
							// refused fallback, a hung one and a slow one were
							// indistinguishable, and only the first is worth
							// waiting for.
							fallbackNote = fmt.Sprintf("System Proxy fallback could not be prepared: %v", fbErr)
							diary.LogErr("connect", "fallback-decide-fail", fbErr)
							e.AddLog("error", fallbackNote, "engine")
						} else {
							e.mu.Lock()
							e.activeAdapter = fbDecision.Adapter
							e.mu.Unlock()
							fbDecision.Adapter.SetLogger(e.AddLog)
							fbDecision.Adapter.SetOnExit(func(exitErr error) {
								e.handleCoreExit(operationID, exitErr, exitFlushNetwork)
							})
							if startErr := fbDecision.Adapter.Start(connCtx, fbDecision.ConfigContent); startErr == nil {
								decision = fbDecision
								adapter = fbDecision.Adapter
								targetProtocol = fbDecision.CoreProtocol
								settings = fallbackSettings
								err = nil
								// Keep e.lastSettings in sync so a later auto-reconnect (which reuses
								// e.lastSettings) retries with "system_proxy", not the original "tun"
								// mode that just failed and triggered this fallback in the first place.
								e.mu.Lock()
								e.lastSettings = fallbackSettings
								e.mu.Unlock()
								diary.Log("connect", "fallback-ok", "core", fbDecision.CoreName)
							} else {
								// Same rule as above: the fallback ran and lost. Its
								// error is a different fault from the TUN failure that
								// triggered it — often the real one — and discarding
								// it left the user with a message about an adapter the
								// fallback had already given up on.
								fallbackNote = fmt.Sprintf("System Proxy fallback (%s) failed to start: %v", fbDecision.CoreName, startErr)
								diary.LogErr("connect", "fallback-start-fail", startErr)
								e.AddLog("error", fallbackNote, "engine")
							}
						}
					}
				}
			}
			if err != nil {
				cancel()
				finalMsg := fmt.Sprintf("Connection failed on %s: %v", decision.CoreName, err)
				if fallbackNote != "" {
					// Say so explicitly: "fell back and the fallback failed" is a
					// different diagnosis from "TUN failed, nothing else was tried".
					finalMsg = fmt.Sprintf("%s — after %s", finalMsg, fallbackNote)
				}
				e.AddLog("error", finalMsg, "engine")
				diary.LogErr("connect", "error", err)
				_ = sysproxy.FlushDNSAndResetNetwork()
				userErr := err.Error()
				if strings.Contains(userErr, "ADMIN_REQUIRED") {
					userErr = "اتصال OpenVPN نیازمند دسترسی Administrator در ویندوز است. لطفاً برنامه را با Run as administrator باز کنید."
				}
				if fallbackNote != "" {
					userErr = fmt.Sprintf("%s (%s)", userErr, fallbackNote)
				}
				e.setErrorStatus(userErr)
				return
			}
		}

		if abortIfCoreDead("post-spawn") {
			return
		}
		if targetProtocol == config.ProtocolOpenVPN {
			// OpenVPN is a Layer-3 TUN adapter tunnel and does not open a local SOCKS port (10808).
			// OpenVPNAdapter.Start has already verified "Initialization Sequence Completed".
			// Verify adapter status without prematurely aborting established tunnels.
			if err := waitForWintunReady(3 * time.Second); err != nil {
				e.AddLog("info", "OpenVPN Layer-3 tunnel confirmed active by core driver", "engine")
			}
		} else {
			timeout := 60 * time.Second
			if targetProtocol == config.ProtocolPsiphon {
				timeout = 240 * time.Second
			}
			if !probeProxy(10808, timeout) {
				adapter.SetOnExit(nil) // keep the explicit error message, not the crash callback
				cancel()
				errMsg := "proxy port 10808 did not become available - core may have failed to start"
				// The core is still running but never opened its inbound — a
				// different failure from "the core died", and the distinction
				// matters when reading a log after the fact.
				diary.Log("connect", "proxy-port-timeout",
					"core", decision.CoreName,
					"port", "10808",
					"window", timeout.String())
				e.AddLog("error", errMsg, "engine")
				if err := sysproxy.ClearSystemProxy(); err != nil {
					e.AddLog("error", fmt.Sprintf("Failed to clear system proxy: %v", err), "engine")
				}
				e.setErrorStatus(errMsg)
				return
			}
		}

		if abortIfCoreDead("post-probe") {
			return
		}

		// End-to-end tunnel verification: an open local port does NOT mean the
		// remote server actually forwards traffic. Probe a real URL through the
		// core before reporting "connected", so dead or ISP-blocked servers
		// surface as an explicit error instead of a misleading connected state.
		// Psiphon is included deliberately. Its tunnel core opens 10808 long before
		// it has a working upstream, and when it has no candidate servers at all it
		// exits without ever touching the port — so "the port is open" is even less
		// meaningful for it than for sing-box. Leaving it out of this check is what
		// let a dead Psiphon session be reported as a successful connection.
		if targetProtocol == config.ProtocolSingBox || targetProtocol == config.ProtocolXray || targetProtocol == config.ProtocolPsiphon {
			testURL := settings.RealDelayUrl
			if testURL == "" {
				testURL = "http://cp.cloudflare.com/generate_204"
			}
			verified := false
			var lastErr error
			verifyWindow := 40 * time.Second
			if targetProtocol == config.ProtocolPsiphon {
				// Psiphon negotiates through its own broker before it can forward
				// anything, which routinely takes longer than a sing-box handshake.
				verifyWindow = 90 * time.Second
			}
			verifyDeadline := time.Now().Add(verifyWindow)
			for time.Now().Before(verifyDeadline) {
				if abortIfCoreDead("verify") {
					return
				}
				if _, verr := parser.RealDelayPing(10808, testURL, 8*time.Second); verr == nil {
					verified = true
					break
				} else {
					lastErr = verr
				}
				time.Sleep(1200 * time.Millisecond)
			}
			if !verified {
				if !isCurrent() {
					return
				}
				// A core that died during verification has already published its own
				// exit error through SetOnExit. Overwriting that with the generic
				// "server is not reachable" text would send the user hunting for a
				// better server when the real fault was local, so the message is
				// only replaced when the core is still alive. The failover below
				// still runs either way — this is the only recovery path available
				// while the state is still StateConnecting.
				e.mu.RLock()
				coreDied := e.lastCoreExitErr != nil
				e.mu.RUnlock()
				adapter.SetOnExit(nil) // keep the explicit error message, not the crash callback
				cancel()
				e.AddLog("error", fmt.Sprintf("Tunnel verification failed through the core: %v", lastErr), "engine")
				diary.LogErr("connect", "verify-fail", lastErr)
				_ = sysproxy.FlushDNSAndResetNetwork()
				if !coreDied {
					e.setErrorStatus("Server is not reachable through the tunnel. " +
						"The profile may be dead, blocked by your ISP, or misconfigured. " +
						"Try another server, update your subscription, or enable Fragment.")
				}

				// A TUN adapter fault is local: it reproduces identically on every
				// server, so failing over walks the whole list while burning the
				// attempt budget and never reports the real cause. Report it once
				// and stop — this is the path that turned "TUN needs Administrator"
				// into "no server works".
				if e.coreDiedFromTunAdapter() {
					diary.Log("connect", "tun-adapter-failed")
					e.AddLog("error", tunNeedsAdminMessage, "engine")
					e.setErrorStatus(tunNeedsAdminMessage)
					return
				}

				// The server, not the machine, is what failed here. Report the
				// verdict so the profile goes into a short cooldown and the next
				// failover, the next "fastest server" and the next automatic retry
				// do not walk straight back onto it. In the 2026-10-11 log a host
				// that refused TCP was retried ~80 times in two minutes.
				e.reportServerHealth(profile.ID, false, fmt.Sprintf("tunnel verification failed: %v", lastErr))

				e.mu.RLock()
				failoverArmed := e.failoverCallback != nil
				e.mu.RUnlock()

				// A dead server on first connect is exactly what auto-failover is
				// for — switch to the best backup profile, bounded by the same
				// attempt counter used for mid-connection drops.
				if (settings.AutoFailover || settings.AutoFailoverEnabled) && failoverArmed && profile.ID != "" {
					maxAttempts := int32(defaultMaxFailoverAttempts)
					if settings.ReconnectAttempts > 0 {
						maxAttempts = int32(settings.ReconnectAttempts)
					}
					if attempts := atomic.AddInt32(&e.failoverAttempts, 1); attempts <= maxAttempts {
						e.AddLog("warning", fmt.Sprintf("[Auto-Failover] Server unreachable (attempt %d/%d). Switching to backup server...", attempts, maxAttempts), "engine")
						go func(pid string) {
							time.Sleep(1 * time.Second)
							e.mu.RLock()
							st := e.status.State
							e.mu.RUnlock()
							if st != StateDisconnected && st != StateDisconnecting {
								e.failoverCallback(pid)
							}
						}(profile.ID)
					} else {
						// The counter is deliberately left where it is. Resetting it
						// here is what made every connect failure read "attempt 1/3"
						// and turned a dead subscription into an unbounded failover
						// loop: the budget is refilled by a successful connection or
						// an explicit Disconnect, not by reaching its ceiling.
						diary.Log("connect", "failover-budget-exhausted",
							"attempts", fmt.Sprintf("%d", attempts),
							"max", fmt.Sprintf("%d", maxAttempts))
						e.AddLog("error", fmt.Sprintf(
							"[Auto-Failover] Max failover attempts (%d) reached. Stopping — refresh the subscription or pick another server; a successful connection or a manual Disconnect resets the budget.",
							maxAttempts), "engine")
					}
				} else {
					// Nothing is going to try another server, and that has to be
					// written down. Six failing connect attempts in the 2026-10-11
					// log produced no failover line at all, which left "failover ran
					// and gave up", "failover was skipped" and "failover is switched
					// off" indistinguishable.
					why := "disabled in settings"
					switch {
					case !(settings.AutoFailover || settings.AutoFailoverEnabled):
						why = "disabled in settings"
					case !failoverArmed:
						why = "no failover handler is registered"
					case profile.ID == "":
						why = "the profile has no id"
					}
					diary.Log("connect", "failover-not-armed", "reason", why)
					e.AddLog("warning", fmt.Sprintf(
						"Auto-Failover was not used (%s): the next connect attempt will try this same server again.", why), "engine")
				}
				return
			}
			e.AddLog("info", "Tunnel verification passed (end-to-end reachability OK)", "engine")
			diary.Log("connect", "verify-ok")
		}

		// The single most important guard in this goroutine. Everything before it
		// is reversible; from here on the pipeline starts changing machine-wide
		// state. Applying the system proxy after the core has already died points
		// every browser and app on the machine at a port nothing listens on — the
		// "connected, but the internet is gone" failure. Re-checked here rather
		// than only at the top because the probe and verification stages can take
		// minutes, which is ample time for the core to exit.
		if abortIfCoreDead("pre-proxy") {
			return
		}
		mode := settings.ProxyMode
		if mode == "" {
			mode = "tun"
		}
		if targetProtocol == config.ProtocolOpenVPN {
			_ = sysproxy.ClearSystemProxy()
			// Report the driver the tunnel is really on. OpenVPN silently retries
			// with tap-windows6 whenever Wintun is unusable, and the previous
			// version announced "Wintun" unconditionally — so the log claimed a
			// Wintun tunnel while the adapter in use was a TAP device
			// (Ethernet 2), which sends anyone debugging a routing problem after
			// the wrong interface.
			driverLabel := "Wintun"
			if d, ok := adapter.(interface{ ActiveDriver() string }); ok {
				if active := d.ActiveDriver(); active != "" && active != "wintun" {
					driverLabel = active
				}
			}
			e.AddLog("info", fmt.Sprintf("OpenVPN Direct TUN Adapter Active (Full-System Routing with %s)", driverLabel), "engine")
		} else {
			// A sing-box TUN adapter already carries the machine's traffic, so the
			// system proxy is a *second* path rather than a requirement — and for
			// some applications a second path is a breakage: they hand non-HTTP
			// traffic to the system HTTP proxy (Telegram's IPv6-only DNS endpoints
			// are the usual example) and an HTTP inbound cannot carry it.
			//
			// "tun_only" drops that extra path. Everything else — a proxy-only core,
			// Xray, Psiphon, where the local ports are the only way out at all —
			// keeps applying it, because skipping it there would leave the machine
			// with no proxy and no tunnel.
			dualRouting := mode == "tun" && targetProtocol == config.ProtocolSingBox
			if dualRouting && settings.TunSystemProxy == "tun_only" {
				_ = sysproxy.ClearSystemProxy()
				e.AddLog("info", fmt.Sprintf(
					"TUN-Only Routing Active: %s (system proxy deliberately not set — apps that ignore the virtual adapter will not be proxied)",
					settings.EffectiveTunInterfaceName()), "engine")
				diary.Log("connect", "routing", "mode", "tun-only")
			} else {
				_ = sysproxy.SetFullSystemProxy(10809, 10808)
				if dualRouting {
					e.AddLog("info", fmt.Sprintf("Dual-Routing Active: TUN Adapter (%s) + System Proxy (127.0.0.1:10809/10808)", settings.EffectiveTunInterfaceName()), "engine")
					diary.Log("connect", "routing", "mode", "dual")
				} else {
					e.AddLog("info", "System Proxy enabled on 127.0.0.1:10809 (HTTP) and 127.0.0.1:10808 (SOCKS5)", "engine")
					diary.Log("connect", "routing", "mode", "system-proxy")
				}
			}
		}

		if mode == "tun" {
			func() {
				time.Sleep(1 * time.Second)
				optimalMTU := settings.CustomMTU
				if settings.AutoMTUEnabled || optimalMTU <= 0 {
					optimalMTU = sysproc.DiscoverOptimalMTU(profile.Server)
				}
				if optimalMTU > 0 {
					if err := sysproc.ApplyWintunMTU(settings.EffectiveTunInterfaceName(), optimalMTU); err == nil {
						e.AddLog("info", fmt.Sprintf("Adaptive MTU applied: %d bytes (MSS Clamped)", optimalMTU), "engine")
					}
				}
			}()
		}

		if abortIfCoreDead("post-mtu") {
			return
		}
		if settings.KillSwitchEnabled {
			if err := GetKillSwitch().Enable(profile.Server); err != nil {
				e.AddLog("error", fmt.Sprintf("Kill Switch configuration failed (Administrator privileges required): %v", err), "engine")
				cancel()
				_ = adapter.Stop()
				_ = sysproxy.ClearSystemProxy()
				// ForceCleanup: Enable may have aborted halfway (e.g. after the
				// global outbound policy was set to block but before the allow
				// rules were all in place), and a partially applied kill switch
				// leaves the machine with no connectivity at all.
				_ = GetKillSwitch().ForceCleanup()
				e.setErrorStatus(fmt.Sprintf("Failed to enable Kill Switch. Administrator privileges are required for Windows Firewall rules: %v", err))
				return
			} else {
				e.AddLog("info", "Kill Switch enabled (Leak prevention active)", "engine")
			}
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

				// Turn a persistently unhealthy tunnel into an actual failover.
				// Without this the "Smart Health Monitor" only produced a log line
				// and the user stayed stuck on a dead server.
				e.mu.RLock()
				activeID := e.activeProfile.ID
				state := e.status.State
				settings := e.lastSettings
				cb := e.failoverCallback
				e.mu.RUnlock()

				if activeID == "" || state != StateConnected || cb == nil {
					return
				}

				if !(settings.AutoFailover || settings.AutoFailoverEnabled) {
					return
				}

				maxAttempts := int32(defaultMaxFailoverAttempts)
				if settings.ReconnectAttempts > 0 {
					maxAttempts = int32(settings.ReconnectAttempts)
				}

				attempts := atomic.AddInt32(&e.failoverAttempts, 1)
				if attempts > maxAttempts {
					// Left saturated on purpose: a successful connection or an
					// explicit Disconnect is what refills the budget, not reaching
					// its ceiling. Zeroing it here let every failure start again at
					// "attempt 1/3" and made a dead subscription an endless loop.
					diary.Log("connect", "health-failover-exhausted",
						"attempts", fmt.Sprintf("%d", attempts),
						"max", fmt.Sprintf("%d", maxAttempts))
					e.AddLog("error", fmt.Sprintf(
						"[Health-Failover] Max failover attempts (%d) reached. Stopping — a successful connection or a manual Disconnect resets the budget.", maxAttempts), "engine")
					return
				}

				e.AddLog("warning", fmt.Sprintf("[Health-Failover] Tunnel unhealthy (attempt %d/%d). Switching to backup server...", attempts, maxAttempts), "engine")
				// The server is being abandoned because it stopped carrying
				// traffic. Recording that keeps the next failover from offering it
				// again as "the fastest backup".
				e.reportServerHealth(activeID, false, fmt.Sprintf("health monitor: %d consecutive failures", fails))
				if settings.NotificationsEnabled {
					go ShowWindowsNotification("NexVPN Health Failover", "Tunnel unhealthy. Switching to best backup server...")
				}
				go func(pid string, cb func(string)) {
					time.Sleep(1 * time.Second)
					e.mu.RLock()
					st := e.status.State
					e.mu.RUnlock()
					if st != StateDisconnected && st != StateDisconnecting {
						cb(pid)
					}
				}(activeID, cb)
			})
			go hm.Start(connCtx)
		}

		if abortIfCoreDead("pre-connected") {
			return
		}
		e.mu.Lock()
		// The guard above reads the state and the exit error under separate locks,
		// so a core that died in between would still slip through. Re-asserting
		// both here, while holding the lock that the transition itself uses, is
		// what makes "reported connected" mean "a live core is forwarding
		// traffic". Without it the crash callback's StateError was simply
		// overwritten by StateConnected a moment later.
		if e.status.State != StateConnecting || e.lastCoreExitErr != nil {
			state := e.status.State
			exitErr := e.lastCoreExitErr
			e.mu.Unlock()
			exitMsg := ""
			if exitErr != nil {
				exitMsg = exitErr.Error()
			}
			diary.Log("connect", "abort-race",
				"state", string(state), "exit", exitMsg)
			_ = sysproxy.ClearSystemProxy()
			_ = adapter.Stop()
			cancel()
			return
		}
		e.healthMonitor = hm
		e.status.State = StateConnected
		e.status.ConnectedAt = time.Now()
		st := e.status
		e.mu.Unlock()

		e.emitStatus(st)
		e.AddLog("info", fmt.Sprintf("Successfully connected to %s", profile.Name), "engine")
		diary.Log("connect", "connected", "core", decision.CoreName, "mode", settings.ProxyMode)

		// Reaching StateConnected is the one piece of evidence that a server
		// carries traffic, for every protocol including the ones with no
		// end-to-end verify step. It clears the profile's failure history and with
		// it the cooldown that kept it out of the candidate lists.
		e.reportServerHealth(profile.ID, true, "")

		// Keep the marker for the lifetime of the session, not just the connect
		// attempt. A process killed while connected leaves the same debris as
		// one killed while connecting, and the marker is the only evidence the
		// next launch has.
		WriteConnectJournal(ConnectJournal{
			Stage:     "connected",
			ProfileID: profile.ID,
			Profile:   profile.Name,
			Protocol:  string(profile.Protocol),
			Server:    profile.Server,
			Mode:      settings.ProxyMode,
			PID:       os.Getpid(),
		})

		// Windows Native Toast Notification
		if settings.NotificationsEnabled {
			go ShowWindowsNotification("NexVPN Connected", fmt.Sprintf("Connected to %s (%s)", profile.Name, profile.Protocol))
		}

		// Real-time traffic sampling loop (bandwidth graph, speeds, session duration,
		// and the byte counters persisted into connection history).
		// NOTE: this must run on connCtx so it stops as soon as the session ends.
		e.startStatsLoop(connCtx, adapter)
	}()

	return nil
}

func (e *Engine) isCurrentOperation(operationID uint64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.operationID == operationID
}

// isConfigDecodeError reports whether a core exited because it could not decode
// the config it was handed.
//
// The bundled core and the generated config schema can drift apart — that is
// exactly what produced `outbounds[2].tls.fragment: cannot unmarshal object ...
// of type bool` — and the resulting failure has nothing to do with TUN. Retrying
// it in System Proxy mode rebuilds the same config and fails identically, so the
// fallback is skipped for these and the real error is reported once.
func isConfigDecodeError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "decode config") ||
		strings.Contains(msg, "cannot unmarshal") ||
		strings.Contains(msg, "unknown field")
}

// tunNeedsAdminMessage is the one TUN failure the user can actually act on.
//
// Kept in Persian because it is shown verbatim as the connection error, and it
// names the fix rather than the symptom: the core's own wording ("Access is
// denied") reads like a bug in the app.
const tunNeedsAdminMessage = "حالت TUN به دسترسی Administrator نیاز دارد — ویندوز ساخت و پیکربندی آداپتور شبکه (Wintun) را فقط به برنامهٔ ارتقایافته اجازه میدهد. " +
	"NexVPN را ببندید و با «Run as administrator» اجرا کنید، یا از حالت System Proxy استفاده کنید."

// isTunAdapterError reports whether a core died because it could not create or
// configure its TUN adapter, rather than because the server was unreachable.
//
// The distinction decides whether failover is worth attempting: an adapter fault
// is local, so it reproduces identically on every server in the list, and
// switching servers only hides the real cause behind a burst of pointless
// reconnects. The core's own final line is included in the exit error (see
// coreErrorSuffix in internal/protocols), which is what makes this detectable.
func isTunAdapterError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "configure tun interface") ||
		strings.Contains(msg, "start inbound/tun") ||
		strings.Contains(msg, "access is denied")
}

// coreDiedFromTunAdapter reports whether the core that just died failed on its
// TUN adapter. Read from the exit callback rather than from the probe error,
// because the probe only ever sees "no traffic passed", which is also what a
// genuinely dead server looks like.
func (e *Engine) coreDiedFromTunAdapter() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return isTunAdapterError(e.lastCoreExitErr)
}

// waitForPortFree waits until a TCP port is released and available for listening.
//
// It reports whether the port became free. The boolean matters: the old version
// returned nothing, so every caller carried on after a timeout and spawned a
// core onto a port that was still held — which is where
// `listen tcp 127.0.0.1:10808: bind: Only one usage of each socket address` came
// from. Probing at least once even with a zero timeout is deliberate.
func waitForPortFree(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = ln.Close()
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// ownCoreImages are the executable base names NexVPN spawns as VPN cores. Only a
// process matching one of these is ever killed by releaseProxyPort: the port
// belongs to this app by convention, but "something is listening on 10808" is
// not by itself a licence to terminate an arbitrary process on the machine.
var ownCoreImages = []string{
	"sing-box.exe", "singbox.exe", "xray.exe", "openvpn.exe",
	"aether.exe", "psiphon.exe", "psiphon-tunnel-core.exe", "nexvpn.exe",
}

func isOwnCoreImage(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, want := range ownCoreImages {
		if name == want || strings.TrimSuffix(name, ".exe") == strings.TrimSuffix(want, ".exe") {
			return true
		}
	}
	return false
}

// releaseProxyPort guarantees that 127.0.0.1:port is genuinely free before a core
// is spawned on it, returning false when it could not be made free.
//
// CleanupStaleCores plus a short wait is not sufficient, and the failure is
// silent in the worst way. `taskkill /F /T` has its own 3 s budget and returns
// before Windows has finished tearing the listener down, so a 1.5 s wait can
// expire while the previous core is still dying. The core then starts on a busy
// port and dies with a FATAL bind error — and because 10808 is also the port the
// health monitor probes and sysproxy points at, the user sees "Connection
// unstable: 3 consecutive health check failures" and the engine fails over onto
// a server that was never the problem.
//
// So: wait, then identify the holder and kill it if it is one of ours, then wait
// again. Never proceed on an unverified port.
func releaseProxyPort(port int, wait time.Duration) bool {
	if waitForPortFree(port, wait) {
		return true
	}

	pid, image, ok := sysproc.ProcessHoldingPort(port)
	if ok && pid > 0 && pid != os.Getpid() && isOwnCoreImage(image) {
		diary.Log("connect", "port-owner-killed",
			"port", strconv.Itoa(port),
			"pid", strconv.Itoa(pid),
			"image", image)
		sysproc.KillProcessTree(pid)
		if waitForPortFree(port, 6*time.Second) {
			return true
		}
	}

	diary.Log("connect", "port-still-busy",
		"port", strconv.Itoa(port),
		"owner", sysproc.DescribeProcessHoldingPort(port))
	return false
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

// TunInterfaceName returns the Wintun adapter name the active connection was
// built with.
//
// Anything that touches the adapter directly — MTU tuning above all — has to ask
// for this rather than assuming the default, because the engine may have had to
// avoid a ghost adapter by using a different name.
func (e *Engine) TunInterfaceName() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastSettings.EffectiveTunInterfaceName()
}

// waitForWintunReady waits for the Wintun/TAP adapter to become active.
func waitForWintunReady(timeout time.Duration) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		psScript := "Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { ($_.Name -like '*OpenVPN*' -or $_.Name -like '*Wintun*' -or $_.Name -eq 'NexVPN-OVPN' -or $_.InterfaceDescription -match 'Wintun|TAP') -and $_.Status -eq 'Up' } | Select-Object -First 1 -ExpandProperty Name"
		psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		psCmd.SysProcAttr = sysproc.HideWindowAttr()
		output, err := psCmd.CombinedOutput()
		if err == nil && len(strings.TrimSpace(string(output))) > 0 {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("wintun adapter not ready within %v", timeout)
}

// Disconnect gracefully stops the active VPN connection and cleans up all system proxy / firewall modifications.
func (e *Engine) Disconnect() error {
	e.mu.Lock()
	if e.status.State == StateDisconnected {
		e.mu.Unlock()
		return nil
	}

	previousState := e.status.State
	e.status.State = StateDisconnecting
	// A deliberate disconnect is the user taking control back, so the retry
	// budget is refilled here — and only here, plus on a successful connection.
	// Refilling it when the ceiling is reached is what made a dead subscription
	// loop through failover and reconnect attempts forever.
	atomic.StoreInt32(&e.failoverAttempts, 0)
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

	_ = sysproxy.FlushDNSAndResetNetwork()
	if err := GetKillSwitch().Disable(); err != nil {
		e.AddLog("error", fmt.Sprintf("Failed to disable Kill Switch: %v", err), "engine")
		// Continue with disconnect process but log the error
	}

	e.emitStatus(st)
	e.AddLog("info", "Disconnecting...", "engine")
	diary.Log("disconnect", "begin",
		"profile", activeP.Name,
		"from_state", string(previousState))

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
	diary.Log("disconnect", "done",
		"profile", activeP.Name,
		"session_sec", fmt.Sprintf("%d", durOrZero(connectedAt)))

	// The state is resolved and this process is still alive to have cleaned up
	// after itself, so there is nothing for the next launch to repair.
	ClearConnectJournal()

	if notifyEnabled {
		go ShowWindowsNotification("NexVPN Disconnected", "Disconnected from server.")
	}

	return nil
}

// durOrZero reports how long the session lasted, or 0 when it never reached
// "connected" (so there is no ConnectedAt to measure from).
func durOrZero(connectedAt time.Time) int64 {
	if connectedAt.IsZero() {
		return 0
	}
	return int64(time.Since(connectedAt).Seconds())
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
			alertBytes := e.sessionDataAlertBytes
			alertFired := e.dataAlertFired
			notificationsOn := e.lastSettings.NotificationsEnabled
			if alertBytes > 0 && !alertFired && (rx+tx) >= alertBytes {
				e.dataAlertFired = true
				alertFired = true
			}
			e.mu.Unlock()

			if alertFired && alertBytes > 0 {
				usedMB := (rx + tx) / (1024 * 1024)
				limitMB := alertBytes / (1024 * 1024)
				e.AddLog("warning", fmt.Sprintf(
					"Session data alert: %d MB transferred, which has reached this profile's %d MB threshold",
					usedMB, limitMB), "engine")
				diary.Log("session", "data-alert",
					"used_mb", strconv.FormatInt(usedMB, 10),
					"limit_mb", strconv.FormatInt(limitMB, 10))
				if notificationsOn {
					go ShowWindowsNotification("NexVPN Data Alert", fmt.Sprintf(
						"This session has passed %d MB of its %d MB threshold.", usedMB, limitMB))
				}
			}

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

	// The failure is being handled by a live process, which is about to run its
	// own cleanup, so the crash-recovery marker is no longer evidence of
	// anything. It is cleared here rather than left for a failover reconnect to
	// overwrite, so that a failover which itself dies still leaves a marker.
	ClearConnectJournal()

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

	maxFailoverAttempts := int32(defaultMaxFailoverAttempts)
	// Use user-configured reconnect attempts if available
	if lastS.ReconnectAttempts > 0 {
		maxFailoverAttempts = int32(lastS.ReconnectAttempts)
	}

	isAutoFailoverEnabled := lastS.AutoFailover || lastS.AutoFailoverEnabled
	// A mid-session drop is charged to the server only when the fault was not
	// local. A core that died because it could not create its TUN adapter fails
	// identically on every server, and cooling servers down for that would both
	// hide the real cause and empty the candidate list.
	if previousState == StateConnected && activeP.ID != "" && !e.coreDiedFromTunAdapter() {
		e.reportServerHealth(activeP.ID, false, "connection dropped mid-session")
	}
	// Only trigger auto-failover if the session was previously established (StateConnected)
	// and then dropped unexpectedly mid-connection, to avoid jumping servers on misconfiguration
	if isAutoFailoverEnabled && previousState == StateConnected && e.failoverCallback != nil && activeP.ID != "" {
		attempts := atomic.AddInt32(&e.failoverAttempts, 1)
		if attempts <= maxFailoverAttempts {
			e.AddLog("warning", fmt.Sprintf("[Auto-Failover] Active connection dropped (attempt %d/%d). Switching to backup server...", attempts, maxFailoverAttempts), "engine")
			if lastS.NotificationsEnabled {
				go ShowWindowsNotification("NexVPN Auto-Failover", "Connection dropped. Switching to best backup server...")
			}
			go func(pid string, cb func(string)) {
				time.Sleep(1 * time.Second)
				// Only failover if still not manually disconnected
				e.mu.RLock()
				st := e.status.State
				e.mu.RUnlock()
				if st != StateDisconnected && st != StateDisconnecting {
					cb(pid)
				}
			}(activeP.ID, e.failoverCallback)
		} else {
			diary.Log("connect", "failover-budget-exhausted",
				"attempts", fmt.Sprintf("%d", attempts),
				"max", fmt.Sprintf("%d", maxFailoverAttempts))
			e.AddLog("error", fmt.Sprintf(
				"[Auto-Failover] Max failover attempts (%d) reached. Stopping — a successful connection or a manual Disconnect resets the budget.", maxFailoverAttempts), "engine")
		}
	} else if autoReconn && previousState == StateConnected && activeP.ID != "" {
		attempts := atomic.AddInt32(&e.failoverAttempts, 1)
		if attempts <= maxFailoverAttempts {
			e.AddLog("warning", fmt.Sprintf("[Auto-Reconnect] Connection dropped (attempt %d/%d). Reconnecting in 3 seconds...", attempts, maxFailoverAttempts), "engine")
			go func(p config.Profile, s config.Settings) {
				time.Sleep(3 * time.Second)
				e.mu.RLock()
				st := e.status.State
				e.mu.RUnlock()
				if st != StateDisconnected && st != StateDisconnecting {
					_ = e.Connect(p, s)
				}
			}(activeP, lastS)
		} else {
			// The counter stays where it is. Refilling the budget by reaching its
			// ceiling is what turned a dead server into an endless reconnect loop.
			diary.Log("connect", "reconnect-budget-exhausted",
				"attempts", fmt.Sprintf("%d", attempts),
				"max", fmt.Sprintf("%d", maxFailoverAttempts))
			e.AddLog("error", fmt.Sprintf(
				"[Auto-Reconnect] Max reconnect attempts (%d) reached. Stopping — a successful connection or a manual Disconnect resets the budget.", maxFailoverAttempts), "engine")
		}
	} else if !isAutoFailoverEnabled && !autoReconn {
		// Neither recovery path is armed, so there is no attempt budget worth
		// keeping and the counter starts clean for the next session.
		//
		// Deliberately NOT resetting just because the session had not reached
		// StateConnected. The verify-fail path in Connect counts its own
		// attempts in this same counter, and zeroing it here made every connect
		// failure report "attempt 1/3" forever — the cap could never be reached,
		// so a dead subscription produced an unbounded failover loop.
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
	} else {
		// Even with Privacy Shield off, credentials carried in a URL query string
		// (subscription tokens, API keys) are never written to the log verbatim.
		message = RedactURLSecrets(message)
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	e.mu.Lock()
	e.logs = append(e.logs, entry)
	if len(e.logs) > maxLogEntries+logTrimBatch {
		// Drop a whole batch instead of shifting one entry per line.
		//
		// The previous `copy(e.logs, e.logs[1:])` ran on *every* log line once
		// the buffer was full — a thousand struct copies per line, under the
		// same mutex the once-a-second status emitter takes, which is where the
		// log view's stutter came from. Trimming in batches makes the cost
		// amortised O(1) while keeping the buffer bounded.
		//
		// The buffer therefore holds up to maxLogEntries+logTrimBatch entries;
		// GetLogs trims to maxLogEntries on read so the visible contract is
		// unchanged.
		n := copy(e.logs, e.logs[logTrimBatch:])
		e.logs = e.logs[:n]
	}
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
	// AddLog trims in batches, so the buffer can briefly hold more than the
	// advertised maximum. Trimming here keeps the caller-visible size fixed.
	src := e.logs
	if len(src) > maxLogEntries {
		src = src[len(src)-maxLogEntries:]
	}
	res := make([]LogEntry, len(src))
	copy(res, src)
	return res
}

// ClearLogs empties the in-memory circular log buffer.
func (e *Engine) ClearLogs() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = make([]LogEntry, 0, 500)
}
