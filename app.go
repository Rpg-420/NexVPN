package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/diary"
	"nexvpn/internal/discovery"
	"nexvpn/internal/dnsscan"
	"nexvpn/internal/sysproxy"
	"nexvpn/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct is the main application backend binding
type App struct {
	// ctx is set once during startup and read from many background goroutines,
	// so every access must go through ctxMu (see appCtx) to avoid a data race.
	ctxMu       sync.RWMutex
	ctx         context.Context
	store       *config.Store
	engine      *core.Engine
	scanner     *core.CleanIPScanner
	subDaemon   *discovery.SubscriptionDaemon
	quotaDaemon *core.QuotaMonitorDaemon

	// netWatchMu guards lastNetEvent, which debounces the burst of interface
	// notifications a single Wi-Fi switch produces into one revalidation.
	netWatchMu   sync.Mutex
	lastNetEvent time.Time

	// pingAllCancel stops a running bulk ping. It is a flag rather than a
	// context because PingAllProfiles is a synchronous binding call: the
	// frontend's promise only settles when it returns, so the only way to give
	// the user a stop button is to have the workers observe a flag and return
	// early.
	pingAllCancel atomic.Bool
}

// appCtx returns the Wails runtime context in a race-free way.
// It returns nil before startup has completed, which callers already guard for.
func (a *App) appCtx() context.Context {
	a.ctxMu.RLock()
	defer a.ctxMu.RUnlock()
	return a.ctx
}

// setAppCtx stores the runtime context once during startup.
func (a *App) setAppCtx(ctx context.Context) {
	a.ctxMu.Lock()
	a.ctx = ctx
	a.ctxMu.Unlock()
}

// NewApp creates a new App instance
func NewApp() *App {
	store := config.NewStore()
	engine := core.NewEngine()
	scanner := core.NewCleanIPScanner()

	app := &App{
		store:   store,
		engine:  engine,
		scanner: scanner,
	}

	// Forward engine logs and status to Wails runtime
	engine.OnStatusChange(func(status core.ConnectionStatus) {
		app.broadcastStatus(status)
	})
	engine.OnLog(func(entry core.LogEntry) {
		app.broadcastLog(entry)
	})
	engine.OnStats(func(stats core.TrafficStats) {
		app.broadcastStats(stats)
	})
	engine.OnHealth(func(health core.HealthStatus) {
		app.broadcastHealth(health)
	})
	engine.OnSessionComplete(func(sess config.ConnectionSession) {
		_ = store.RecordConnectionSession(sess)
		if app.ctx != nil {
			runtime.EventsEmit(app.ctx, "session:recorded", sess)
		}
	})
	engine.OnFailover(func(failedProfileID string) {
		app.triggerAutoFailover(failedProfileID)
	})

	// The engine is what proves a server reachable or unreachable — the store is
	// what remembers it. Recording the verdict as a per-profile cooldown keeps
	// failover, "fastest server" and the automatic retries from walking back onto
	// a host that refused a connection seconds earlier, and it survives an app
	// restart because it lives in the profile rather than in memory.
	engine.OnServerHealth(func(profileID string, healthy bool, reason string) {
		if store == nil || profileID == "" {
			return
		}
		if healthy {
			if err := store.ClearProfileFailure(profileID); err != nil {
				diary.Log("store", "clear-server-failure-failed", "profile", profileID, "err", err.Error())
			}
			return
		}
		if err := store.RecordProfileFailure(profileID, reason); err != nil {
			diary.Log("store", "record-server-failure-failed", "profile", profileID, "err", err.Error())
		}
	})

	return app
}

func (a *App) startup(ctx context.Context) {
	a.setAppCtx(ctx)
	a.engine.SetContext(ctx)

	// Repair the debris of a previous run that did not exit cleanly, before
	// anything else touches the network stack. The generic stale-cleanup below
	// handles the common cases; this adds the ones only a killed process leaves
	// behind (a system proxy pointing at a dead port, a ghost TUN adapter, a
	// half-applied kill switch).
	a.runStartupRepair()

	// Clean any dangling proxy, firewall rules, or core processes left by a crash or previous session.
	// ForceCleanup, not Disable: a force-killed previous run can leave the global
	// outbound policy set to "block" with no in-process record of it, and this is
	// the only place that repairs that.
	sysproxy.CheckAndClearStaleProxy()
	_ = core.GetKillSwitch().ForceCleanup()
	core.CleanupStaleCores()
	diary.Log("launch", "stale-cleanup-done")

	// Initialize Windows Taskbar Notification Area (System Tray) icon near the clock
	tray.SetToggleConnectHandler(func() {
		status := a.GetConnectionStatus()
		if status.State == "connected" || status.State == "connecting" {
			_ = a.Disconnect()
		} else {
			_, _ = a.ConnectFastest("all")
		}
	})
	tray.SetFastConnectHandler(func() {
		_, _ = a.ConnectFastest("all")
	})
	tray.SetPanicWipeHandler(func() {
		_ = a.PanicWipe()
	})
	tray.SetShowHandler(func() {
		if a.appCtx() != nil {
			runtime.WindowShow(a.appCtx())
			runtime.WindowUnminimise(a.appCtx())
		}
		tray.ShowMainWindow()
	})
	tray.SetExitHandler(func() {
		if a.appCtx() != nil {
			runtime.Quit(a.appCtx())
		}
	})
	// The tray's server submenu: pinned profiles first, then the most recently
	// used. Selecting one connects to it without bringing the window forward,
	// which is the entire point of having the list there.
	tray.SetServerListProvider(a.trayServerItems)
	tray.SetServerSelectHandler(func(profileID string) {
		if err := a.Connect(profileID); err != nil {
			a.engine.AddLog("error", fmt.Sprintf("Tray connect failed: %v", err), "tray")
		}
	})
	tray.InitSystemTray("NexVPN Pro")
	tray.ApplyMainWindowIcon()
	diary.Log("launch", "tray-ready")

	// Load store data
	if err := a.store.Load(); err != nil {
		diary.LogErr("launch", "store-load-failed", err)
		fmt.Printf("Failed to load store: %v\n", err)
	}
	diary.Log("launch", "store-loaded", "profiles", fmt.Sprintf("%d", len(a.store.GetProfiles())))

	// Registered after the store loads, because the menu labels itself from the
	// persisted rendering mode — reading it before Load would show the default and
	// offer the action that is already in effect.
	a.registerTrayDiagnostics()

	// Sync auto-reconnect setting
	settings := a.store.GetSettings()
	a.engine.SetAutoReconnect(settings.AutoReconnect)

	// Apply the persisted core-download integrity policy. Off by default, so a
	// compromised mirror cannot substitute an unverified binary unless the
	// user deliberately opted in.
	core.SetAllowUnverifiedCoreDownloads(settings.AllowUnverifiedCoreDownloads)

	// Start autonomous subscription daemon
	a.subDaemon = discovery.NewSubscriptionDaemon(a.store, a.engine.AddLog)
	if settings.SubscriptionAutoUpdateHours > 0 {
		a.subDaemon.SetInterval(time.Duration(settings.SubscriptionAutoUpdateHours) * time.Hour)
	}
	a.subDaemon.Start()

	// Start background quota monitor (checks remaining data / expiry and
	// dispatches the configured webhook alert).
	a.quotaDaemon = core.NewQuotaMonitorDaemon(a.store, a.engine.AddLog)
	a.quotaDaemon.Start()

	// OS event watchers (network change, sleep/resume, mains power) plus the
	// scheduled connect/disconnect ticker. They are started after the daemons
	// so that a network event arriving immediately cannot race with store
	// loading, and they are pure latency improvements over the health monitor.
	a.startWatchdogs(ctx)
	diary.Log("launch", "watchdogs-started")

	// StartAutoQuarantineService is deliberately NOT called. It has been a no-op
	// since hourly background probing was retired, and calling it did nothing
	// except push a deprecation warning into every launch log. The bound method
	// stays in place for older frontends.
	diary.Log("launch", "daemons-started")

	// Check if started minimized via command-line or settings
	startMinimized := settings.StartMinimized
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(arg, "--minimized") || strings.EqualFold(arg, "-minimized") {
			startMinimized = true
			break
		}
	}
	if startMinimized && a.appCtx() != nil {
		runtime.WindowHide(a.appCtx())
	}
	diary.Log("launch", "ready", "start_minimized", fmt.Sprintf("%t", startMinimized))

	// Auto-connect if enabled
	if settings.AutoConnect {
		diary.Log("launch", "auto-connect-queued", "last_profile", settings.LastProfileID)
		go func() {
			time.Sleep(1 * time.Second)
			targetID := settings.LastProfileID
			if targetID != "" {
				if _, err := a.store.GetProfile(targetID); err != nil {
					targetID = ""
				}
			}
			if targetID == "" {
				profiles := a.store.GetProfiles()
				if len(profiles) > 0 {
					targetID = profiles[0].ID
				}
			}
			if targetID != "" {
				_ = a.Connect(targetID)
			}
		}()
	}
}

func (a *App) shutdown(ctx context.Context) {
	tray.RemoveSystemTray()
	if a.subDaemon != nil {
		a.subDaemon.Stop()
	}
	if a.quotaDaemon != nil {
		a.quotaDaemon.Stop()
	}
	if a.engine != nil {
		_ = a.engine.Disconnect()
	}
	sysproxy.ClearSystemProxy()
	_ = core.GetKillSwitch().ForceCleanup()
	core.CleanupStaleCores()

	// This run cleaned up after itself, so the crash-recovery marker must not
	// survive to make the next launch repair state that is already fine.
	core.ClearConnectJournal()
}

// ============================================================
// Connection Management
// ============================================================

func (a *App) Connect(profileID string) error {
	profile, err := a.store.GetProfile(profileID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	settings := a.store.GetSettings()

	// Fix #5: Warn if Xray is selected but App split tunneling is configured
	// (Xray does not support process-based routing)
	if settings.V2RayCore != "singbox" &&
		(settings.AppRoutingMode == "bypass_selected" || settings.AppRoutingMode == "proxy_selected") &&
		len(settings.SelectedApps) > 0 {
		profile2 := profile
		// Only warn for protocols that might use Xray
		if profile2.Protocol != config.ProtocolWireGuard &&
			profile2.Protocol != config.ProtocolWARP &&
			profile2.Protocol != config.ProtocolHysteria2 &&
			profile2.Protocol != config.ProtocolOpenVPN {
			a.engine.AddLog("warning",
				"App split tunneling is only supported in Sing-box TUN mode. "+
					"Set V2Ray Core to 'singbox' or switch to TUN mode for this feature to work.",
				"app")
		}
	}

	// TunnelBear profiles are rewritten here rather than stored ready. The
	// relay's loopback port, the addresses the operator's hostname resolves to
	// right now, and the account the user has selected all exist only at this
	// moment — baking any of them into the saved profile would make it stale
	// the first time the relay restarted or the account changed.
	//
	// The rewrite lands on `connectProfile`; `profile` stays exactly what came
	// out of the store. That separation is load-bearing: the bookkeeping write
	// at the end of this function persists `profile`, and if the rewrite had
	// been applied to it, config.json would end up holding the relay's
	// ephemeral port and the account's plaintext password — precisely what the
	// account vault exists to prevent.
	connectProfile := profile
	tunnelBearAccount := ""
	if profile.Protocol == config.ProtocolOpenVPN && tunnelBearHostOf(profile) != "" {
		prepared, accountID, prepErr := a.prepareTunnelBearProfile(profile)
		if prepErr != nil {
			diary.LogErr("connect", "tunnelbear-prepare-failed", prepErr)
			return prepErr
		}
		connectProfile = prepared
		tunnelBearAccount = accountID
	}

	if err := a.engine.Connect(connectProfile, settings); err != nil {
		if tunnelBearAccount != "" {
			// A TunnelBear failure is worth attributing to the account, but only
			// when the core actually said the credentials were rejected —
			// marking a timeout as an authentication failure would send the user
			// to re-enter a password that was never the problem.
			if strings.Contains(err.Error(), "AUTH_FAILED") {
				discovery.NoteTunnelBearAuthResult(tunnelBearAccount, discovery.TunnelBearAuthFailed, err.Error())
			}
			// Whatever the reason, nothing is going to use the relay now.
			closeTunnelBearRelay()
		}
		return err
	}

	if tunnelBearAccount != "" {
		// Reaching here means the tunnel came up, which means the operator's
		// servers accepted the credentials.
		discovery.NoteTunnelBearAuthResult(tunnelBearAccount, discovery.TunnelBearAuthOK, "")
	}

	settings.LastProfileID = profileID
	profile.UsageCount++
	profile.LastUsed = time.Now()
	_ = a.store.UpdateProfile(profile)
	_ = a.store.SaveSettings(settings)

	return nil
}

func (a *App) Disconnect() error {
	// The relay exists to carry one connection. Leaving it listening would hold
	// a loopback port and a socket to a server the user has just left.
	closeTunnelBearRelay()
	return a.engine.Disconnect()
}

// failoverProbeCandidates bounds how many backups are probed before a failover.
// Probing all of a large subscription would take longer than the failover is
// worth; the fastest few are the only realistic winners.
const failoverProbeCandidates = 6

// failoverProbeBudget bounds the whole probe. The callback runs on its own
// goroutine, so blocking here is safe, but the user is staring at a spinner
// while it happens.
const failoverProbeBudget = 4 * time.Second

// triggerAutoFailover picks a backup server and connects to it.
//
// The backup is probed before it is used. Previously the choice was made purely
// from the cached LatencyMs field, which is a number from whenever the server
// was last measured — it says nothing about whether the server is alive *now*.
// In the TUN crash-loop incident that produced four consecutive failovers onto
// an independently dead WARP/WireGuard backend: the attempt budget was spent,
// the real cause (a local adapter fault) stayed hidden, and the user was left
// watching a spinner. When nothing answers, saying so is more useful than
// switching to another dead server.
func (a *App) triggerAutoFailover(failedProfileID string) {
	// Every exit from this function is recorded. The 2026-10-09 log showed an
	// "[Auto-Failover] Server unreachable" warning in the app log with nothing
	// at all in the diary afterwards, which made "failover ran and gave up",
	// "failover was skipped" and "the callback never fired" indistinguishable.
	diary.Log("connect", "failover-begin", "failed_profile", failedProfileID)

	if a.store == nil {
		diary.Log("connect", "failover-skip", "reason", "no-store")
		return
	}
	profiles := a.store.GetProfiles()
	if len(profiles) <= 1 {
		diary.Log("connect", "failover-skip",
			"reason", "no-other-profiles",
			"profiles", fmt.Sprintf("%d", len(profiles)))
		return
	}

	candidates := core.RankFailoverCandidates(profiles, failedProfileID, failoverProbeCandidates)
	if len(candidates) == 0 {
		diary.Log("connect", "failover-skip",
			"reason", "no-candidates",
			"profiles", fmt.Sprintf("%d", len(profiles)))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), failoverProbeBudget)
	reachable := core.ProbeReachableProfiles(ctx, candidates)
	// Release the probe context before connecting: Connect can block for a
	// while, and there is no reason to keep a timeout armed over it.
	cancel()

	if len(reachable) > 0 {
		target := reachable[0].Profile
		diary.Log("connect", "failover-probe-ok",
			"reachable", fmt.Sprintf("%d/%d", len(reachable), len(candidates)),
			"target", target.Name,
			"latency_ms", fmt.Sprintf("%d", reachable[0].LatencyMs))

		a.engine.AddLog("info", fmt.Sprintf(
			"[Auto-Failover] %d of %d backup servers answered; switching to the fastest: %s (%s, %dms)",
			len(reachable), len(candidates), target.Name, target.Protocol, reachable[0].LatencyMs), "engine")

		_ = a.Connect(target.ID)
		return
	}

	// Nothing we could dial answered. Profiles whose endpoint is buried inside a
	// pasted config (an imported .ovpn, a WireGuard file whose server could not
	// be parsed) can never pass a probe, and for those we genuinely do not know
	// whether they are alive — one blind attempt beats doing nothing, as long as
	// we say that is what we are doing.
	if fallbacks := core.UnprobeableCandidates(candidates); len(fallbacks) > 0 {
		target := fallbacks[0]
		diary.Log("connect", "failover-blind-attempt",
			"probed", fmt.Sprintf("%d", len(candidates)-len(fallbacks)),
			"target", target.Name)

		a.engine.AddLog("warning", fmt.Sprintf(
			"[Auto-Failover] None of the %d testable backup servers answered. Trying %s anyway — "+
				"its address cannot be checked before connecting.",
			len(candidates)-len(fallbacks), target.Name), "engine")

		_ = a.Connect(target.ID)
		return
	}

	cooling := core.CountCoolingDown(profiles)
	diary.Log("connect", "failover-none-reachable",
		"probed", fmt.Sprintf("%d", len(candidates)),
		"cooling_down", fmt.Sprintf("%d", cooling))
	msg := fmt.Sprintf(
		"[Auto-Failover] None of the %d backup servers answered a reachability probe. "+
			"Staying disconnected instead of switching to a server that is likely dead too — "+
			"the problem is more likely your network or ISP than this particular profile.",
		len(candidates))
	if cooling > 0 {
		// Without this the message reads "everything is dead", when what actually
		// happened is that everything already failed minutes ago and is being
		// avoided on purpose. Refreshing the subscription is the fix in that case,
		// and the user needs to be told which of the two they are looking at.
		msg += fmt.Sprintf(" %d of them had already failed in the last few minutes and are on a short cooldown; update the subscription if this keeps happening.", cooling)
	}
	a.engine.AddLog("error", msg, "engine")
}

func (a *App) GetConnectionStatus() core.ConnectionStatus {
	return a.engine.GetStatus()
}

func (a *App) GetTrafficStats() core.TrafficStats {
	return a.engine.GetStats()
}

// UsageReport is the honest answer to "how much of my monthly quota have I
// actually used?".
//
// Previously the Quota Manager compared the *current session's* byte counters
// against the configured monthly allowance, so a fresh connection always looked
// like it had barely used anything. This aggregates real per-session totals
// from the connection history over a defined window, and states which window it
// used so the UI can label it accurately.
type UsageReport struct {
	// WindowStart / WindowEnd bound the measured period (UTC).
	WindowStart string `json:"window_start"`
	WindowEnd   string `json:"window_end"`
	// WindowKind is "calendar_month" or "rolling_30d".
	WindowKind string `json:"window_kind"`
	// BytesFromHistory is the sum of every recorded session in the window.
	BytesFromHistory int64 `json:"bytes_from_history"`
	// BytesCurrentSession is the live session's counters, which are NOT yet
	// included in the history (a session is recorded on disconnect).
	BytesCurrentSession int64 `json:"bytes_current_session"`
	// TotalBytes is BytesFromHistory + BytesCurrentSession.
	TotalBytes int64 `json:"total_bytes"`
	// SessionCount is how many recorded sessions fall inside the window.
	SessionCount int `json:"session_count"`
	// QuotaBytes is the configured allowance; 0 means "not configured".
	QuotaBytes int64 `json:"quota_bytes"`
	// UsedPercent is TotalBytes / QuotaBytes, or 0 when no quota is set.
	UsedPercent float64 `json:"used_percent"`
	// HistoryAvailable is false when no session history exists at all, so the
	// UI can distinguish "nothing used" from "nothing recorded".
	HistoryAvailable bool `json:"history_available"`
}

// GetUsageReport aggregates month-to-date traffic from the connection history.
func (a *App) GetUsageReport() UsageReport {
	now := time.Now().UTC()
	// Calendar month to date: usage-based plans bill per calendar month, so
	// this is the window the user's quota number refers to.
	windowStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	windowEnd := now

	report := UsageReport{
		WindowStart: windowStart.Format(time.RFC3339),
		WindowEnd:   windowEnd.Format(time.RFC3339),
		WindowKind:  "calendar_month",
	}

	if a.store != nil {
		history := a.store.GetConnectionHistory()
		report.HistoryAvailable = len(history) > 0
		for _, s := range history {
			// Attribute a session to the window by when it started. A session
			// that began before the window but is still running is not counted
			// here; its bytes land in the next window via the live counters.
			if s.ConnectedAt.Before(windowStart) || s.ConnectedAt.After(windowEnd) {
				continue
			}
			report.BytesFromHistory += s.BytesReceived + s.BytesSent
			report.SessionCount++
		}
		settings := a.store.GetSettings()
		if settings.MonthlyQuotaGB > 0 {
			report.QuotaBytes = int64(settings.MonthlyQuotaGB) * 1024 * 1024 * 1024
		}
	}

	if a.engine != nil {
		stats := a.engine.GetStats()
		report.BytesCurrentSession = stats.BytesReceived + stats.BytesSent
	}

	report.TotalBytes = report.BytesFromHistory + report.BytesCurrentSession
	if report.QuotaBytes > 0 {
		report.UsedPercent = float64(report.TotalBytes) / float64(report.QuotaBytes) * 100
		if report.UsedPercent > 100 {
			report.UsedPercent = 100
		}
	}
	return report
}

// ============================================================
// Settings & System Management
// ============================================================

func (a *App) ScanAndSelectBestDNS(useDoH bool) []interface{} { // Returns generic slice for Wails binding simplicity
	results := dnsscan.ScanAll(useDoH)

	// Convert to generic map for frontend
	var out []interface{}
	for _, r := range results {
		out = append(out, map[string]interface{}{
			"server_name": r.Server.Name,
			"address":     r.Server.Address,
			"is_doh":      r.Server.IsDoH,
			"latency":     r.Latency,
			"is_success":  r.IsSuccess,
			"raw_value":   r.RawValue,
		})
	}
	return out
}

func (a *App) GetSettings() config.Settings {
	if a.store == nil {
		return config.Settings{}
	}
	return a.store.GetSettings()
}

func (a *App) SaveSettings(settings config.Settings) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	_ = core.SetAutoStart(settings.StartOnBoot)
	a.engine.SetAutoReconnect(settings.AutoReconnect)
	// Apply a changed subscription auto-update interval to the running daemon
	// immediately (previously this only took effect after an app restart).
	if a.subDaemon != nil && settings.SubscriptionAutoUpdateHours > 0 {
		a.subDaemon.SetInterval(time.Duration(settings.SubscriptionAutoUpdateHours) * time.Hour)
	}
	// Ensure the quota watcher is running and re-check right away when the user
	// changes the alert threshold or webhook URL.
	if a.quotaDaemon != nil {
		if !a.quotaDaemon.IsRunning() {
			a.quotaDaemon.Start()
		}
		go a.quotaDaemon.CheckAll()
	}
	// Re-apply the core-download integrity policy so toggling it takes effect
	// without an app restart.
	core.SetAllowUnverifiedCoreDownloads(settings.AllowUnverifiedCoreDownloads)
	return a.store.SaveSettings(settings)
}

func (a *App) SetProxyMode(mode string) error {
	settings := a.store.GetSettings()
	settings.ProxyMode = mode
	return a.store.SaveSettings(settings)
}

// ============================================================
// Logging
// ============================================================

func (a *App) GetLogs() []core.LogEntry {
	return a.engine.GetLogs()
}

func (a *App) ClearLogs() {
	a.engine.ClearLogs()
}

// GetStartupDiaryPath returns the on-disk launch diary. Unlike the in-memory log
// buffer, this file survives a crash or a hang before the window is drawn, so
// the UI surfaces it as the thing to attach to a bug report.
func (a *App) GetStartupDiaryPath() string {
	return diary.Path()
}

func (a *App) GetVersion() string {
	return AppVersion
}

// UpdateCheckResult is the result of a GitHub Releases update check
type UpdateCheckResult struct {
	HasUpdate      bool   `json:"has_update"`
	LatestVersion  string `json:"latest_version"`
	CurrentVersion string `json:"current_version"`
	ReleaseURL     string `json:"release_url"`
	ReleaseNotes   string `json:"release_notes"`
	Error          string `json:"error,omitempty"`
}

// CheckForUpdates queries GitHub Releases API to compare latest version with current
func (a *App) CheckForUpdates() UpdateCheckResult {
	const repoAPI = "https://api.github.com/repos/Rpg-420/NexVPN/releases/latest"

	client := &http.Client{Timeout: 8 * time.Second}
	reqCtx := a.appCtx()
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	req, err := http.NewRequestWithContext(reqCtx, "GET", repoAPI, nil)
	if err != nil {
		return UpdateCheckResult{Error: "Failed to build request: " + err.Error(), CurrentVersion: AppVersion}
	}
	req.Header.Set("User-Agent", "NexVPN/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateCheckResult{Error: "Network error: " + err.Error(), CurrentVersion: AppVersion}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// No release found at all — we are on latest
		return UpdateCheckResult{
			HasUpdate:      false,
			LatestVersion:  AppVersion,
			CurrentVersion: AppVersion,
			ReleaseURL:     "https://github.com/Rpg-420/NexVPN/releases",
		}
	}

	if resp.StatusCode != 200 {
		return UpdateCheckResult{Error: "GitHub API returned status " + resp.Status, CurrentVersion: AppVersion}
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateCheckResult{Error: "Failed to parse response: " + err.Error(), CurrentVersion: AppVersion}
	}

	// Normalize tag (strip leading "v")
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(AppVersion, "v")

	notes := release.Body
	if len(notes) > 300 {
		// UTF-8 safe truncation
		runes := []rune(notes)
		if len(runes) > 300 {
			notes = string(runes[:300]) + "..."
		}
	}

	hasUpdate := latest != "" && latest != current

	return UpdateCheckResult{
		HasUpdate:      hasUpdate,
		LatestVersion:  latest,
		CurrentVersion: current,
		ReleaseURL:     release.HTMLURL,
		ReleaseNotes:   notes,
	}
}
