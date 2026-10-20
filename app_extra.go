package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	osruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
	"nexvpn/internal/tray"
)

var (
	scanMu      sync.Mutex
	scanCancel  context.CancelFunc
	speedMu     sync.Mutex
	speedCancel context.CancelFunc
)

// ShowWindow brings the window to the front
func (a *App) ShowWindow() {
	if a.appCtx() != nil {
		runtime.WindowShow(a.appCtx())
		runtime.WindowUnminimise(a.appCtx())
	}
	tray.ShowMainWindow()
}

// HideWindow hides the window to the system tray
func (a *App) HideWindow() {
	if a.appCtx() != nil {
		runtime.WindowHide(a.appCtx())
	}
}

// SetTitle sets the window title. The WebView must already be initialised, so
// this is a no-op before startup rather than a nil-pointer panic.
func (a *App) SetTitle(title string) {
	if ctx := a.appCtx(); ctx != nil {
		runtime.WindowSetTitle(ctx, title)
	}
}

func (a *App) broadcastStatus(status core.ConnectionStatus) {
	tray.UpdateTrayState(status.State == "connected", status.ProfileName, 0)
	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "status:changed", status)
	}
}

// SetTrayLanguage updates the active language in the system tray menu
func (a *App) SetTrayLanguage(lang string) error {
	tray.SetTrayLanguage(lang)
	return nil
}

func (a *App) broadcastLog(entry core.LogEntry) {
	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "log:entry", entry)
	}
}

func (a *App) broadcastStats(stats core.TrafficStats) {
	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "stats:update", stats)
	}
}

func (a *App) broadcastHealth(health core.HealthStatus) {
	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "health:update", health)
	}
}

func (a *App) GetProfileByID(id string) (config.Profile, error) {
	if a.store == nil {
		return config.Profile{}, fmt.Errorf("store not initialized")
	}
	return a.store.GetProfile(id)
}

func (a *App) GetAppDataPath() string {
	return a.store.GetDataDir()
}

func (a *App) OpenProfilesFolder() {
	if a.store != nil {
		dataDir := a.store.GetDataDir()
		runtime.BrowserOpenURL(a.appCtx(), "file://"+dataDir)
	}
}

func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"app_version": AppVersion,
		"go_version":  osruntime.Version(),
		"platform":    osruntime.GOOS,
		"arch":        osruntime.GOARCH,
	}
}

func (a *App) CheckCoreBinaries() map[string]core.BinaryInfo {
	return core.CheckBinaries()
}

func (a *App) InstallCoreBinary(coreKey string) error {
	if a.engine != nil {
		st := a.engine.GetStatus()
		if st.State == core.StateConnected || st.State == core.StateConnecting {
			return fmt.Errorf("cannot update core engine while VPN connection is active; please disconnect first")
		}
	}
	// Use UpdateCoreEngine which handles download + extraction
	_, err := core.UpdateCoreEngine(coreKey)
	return err
}

// StartCleanIPScan begins scanning Cloudflare/WARP IP ranges concurrently across specified ports
func (a *App) StartCleanIPScan(customSubnets []string, customPorts []int) {
	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	scanCancel = cancel
	scanMu.Unlock()

	go func() {
		defer cancel()

		results, err := parser.ScanCleanIPs(ctx, customSubnets, customPorts, 35, func(p parser.ScanProgress) {
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "scanner:progress", p)
			}
		})

		// Always emit a terminal event — including on cancellation — otherwise the
		// frontend spinner/stop button stays stuck forever after a user-initiated Stop.
		if a.appCtx() == nil {
			return
		}
		if err == nil {
			runtime.EventsEmit(a.appCtx(), "scanner:complete", results)
		} else {
			runtime.EventsEmit(a.appCtx(), "scanner:error", err.Error())
		}
	}()
}

type BatchIPRequest struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// BatchGenerateWarpProfiles creates multiple WARP profiles for selected clean IPs.
// When useAmnezia is true the AmneziaWG obfuscation parameters from Settings are
// embedded into each generated profile so the fleet-wide tuning values actually
// reach the generated config (previously only the display name changed).
func (a *App) BatchGenerateWarpProfiles(targets []BatchIPRequest, useAmnezia bool) ([]config.Profile, error) {
	amz := parser.AmneziaParams{}
	if useAmnezia && a.store != nil {
		s := a.store.GetSettings()
		amz = parser.AmneziaParams{
			Jc:   s.AmneziaJc,
			Jmin: s.AmneziaJmin,
			Jmax: s.AmneziaJmax,
			S1:   s.AmneziaS1,
			S2:   s.AmneziaS2,
			H1:   s.AmneziaH1,
			H2:   s.AmneziaH2,
			H3:   s.AmneziaH3,
			H4:   s.AmneziaH4,
		}
	}

	var created []config.Profile
	for i, t := range targets {
		if t.IP == "" {
			continue
		}
		p, err := parser.GenerateCloudflareWarpWithAmnezia(t.IP, t.Port, amz)
		if err != nil {
			continue
		}
		if useAmnezia {
			p.Name = fmt.Sprintf("AmneziaWG WARP - %s:%d", t.IP, t.Port)
			p.Group = "AmneziaWG WARP"
		} else {
			p.Name = fmt.Sprintf("WARP Clean %d (%s:%d)", i+1, t.IP, t.Port)
			p.Group = "Cloudflare WARP"
		}
		id, err := a.store.AddProfile(*p)
		if err == nil {
			p.ID = id
			created = append(created, *p)
		}
	}
	return created, nil
}

// StopCleanIPScan cancels ongoing IP scanner
func (a *App) StopCleanIPScan() {
	scanMu.Lock()
	defer scanMu.Unlock()
	if scanCancel != nil {
		scanCancel()
		scanCancel = nil
	}
}

// StartRealSpeedTest executes a real download/upload speed test through the active proxy
func (a *App) StartRealSpeedTest() {
	speedMu.Lock()
	if speedCancel != nil {
		speedCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	speedCancel = cancel
	speedMu.Unlock()

	// The HTTP proxy inbound always listens on 10809 (10808 is the SOCKS port),
	// so the speed test targets 10809 in every proxy mode. The previous
	// `if settings.ProxyMode == "system_proxy" { port = 10809 }` branch assigned
	// the same value it already had and only obscured the intent.
	const speedTestPort = 10809

	go func() {
		defer cancel()

		res, err := parser.RunSpeedTest(ctx, speedTestPort, func(p parser.SpeedTestProgress) {
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "speedtest:progress", p)
			}
		})
		if a.appCtx() == nil {
			return
		}
		if err == nil && res != nil {
			runtime.EventsEmit(a.appCtx(), "speedtest:complete", *res)
		} else if err != nil {
			runtime.EventsEmit(a.appCtx(), "speedtest:error", err.Error())
		}
	}()
}

// StopRealSpeedTest cancels running speed test
func (a *App) StopRealSpeedTest() {
	speedMu.Lock()
	defer speedMu.Unlock()
	if speedCancel != nil {
		speedCancel()
		speedCancel = nil
	}
}

// ApplyWarpLicenseKey validates a WARP+ license key, stores it, and regenerates
// the WARP account.
//
// A license key is a 26-character base32 string (letters A-Z and digits 2-7,
// conventionally grouped as xxxxxxxx-xxxxxxxx-xxxxxxxx). Anything else is
// rejected so an arbitrary string can no longer be persisted as a paying account.
func (a *App) ApplyWarpLicenseKey(licenseKey string) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}

	clean := strings.ToUpper(strings.TrimSpace(licenseKey))
	clean = strings.ReplaceAll(clean, "-", "")

	if clean == "" {
		return fmt.Errorf("license key is empty")
	}
	if len(clean) != 26 {
		return fmt.Errorf("invalid WARP+ license key: expected 26 characters, got %d", len(clean))
	}
	for _, r := range clean {
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '2' && r <= '7'
		if !isUpper && !isDigit {
			return fmt.Errorf("invalid WARP+ license key: character %q is not allowed (only A-Z and 2-7)", r)
		}
	}

	settings := a.store.GetSettings()
	settings.WarpLicenseKey = licenseKey
	settings.WarpAccountType = "plus"
	if err := a.store.SaveSettings(settings); err != nil {
		return fmt.Errorf("failed to persist WARP+ license key: %w", err)
	}

	// Regenerate Warp profile with clean IP
	cleanIP := settings.WarpCleanIP
	if cleanIP == "" {
		cleanIP = "162.159.192.1"
	}
	_, err := a.GenerateWarpProfile(cleanIP, settings.WarpPort)
	return err
}

// StartSubscriptionScheduler is retained as an explicit no-op for API
// compatibility with older frontends.
//
// Subscription auto-update is already owned by discovery.SubscriptionDaemon,
// which is started once in NewApp and whose interval is updated by SaveSettings.
// The previous implementation started a *second*, independent ticker, so calling
// it doubled the update cadence and leaked a goroutine per call.
func (a *App) StartSubscriptionScheduler() {
	if a.engine != nil {
		a.engine.AddLog("warn", "StartSubscriptionScheduler is deprecated: subscription updates are managed by the built-in daemon.", "app")
	}
}

// ConnectFastest finds the lowest latency healthy profile and connects to it
func (a *App) ConnectFastest(category string) (*config.Profile, error) {
	var profiles []config.Profile
	if category != "" && category != "all" {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	} else {
		profiles = a.store.GetProfiles()
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found to connect")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	best, _, err := core.FindFastestProfile(ctx, profiles, "http://cp.cloudflare.com/generate_204")
	if err != nil || best == nil {
		// Do not silently connect to an arbitrary profile: if latency probing
		// failed we cannot claim this is the "fastest" one, so surface the reason.
		return nil, fmt.Errorf("could not determine the fastest profile: %w", err)
	}

	err = a.Connect(best.ID)
	if err != nil {
		return nil, err
	}

	return best, nil
}

// RunNetworkDiagnostics runs automated network and driver health tests
func (a *App) RunNetworkDiagnostics() parser.DoctorReport {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return parser.RunNetworkDoctor(ctx)
}

// FixNetworkIssues runs 1-click automatic recovery for DNS and proxy
func (a *App) FixNetworkIssues() parser.RepairResult {
	return parser.RepairNetworkIssues()
}

// GetInstalledApplications returns Windows app catalog with running status
func (a *App) GetInstalledApplications() []sysproxy.InstalledApp {
	return sysproxy.GetDetectedApplications()
}

// SaveAppRouting updates split tunneling application rules in settings
func (a *App) SaveAppRouting(mode string, apps []string) error {
	settings := a.store.GetSettings()
	settings.AppRoutingMode = mode
	settings.SelectedApps = apps
	return a.store.SaveSettings(settings)
}

// FetchSubscriptionInfo queries subscription headers to extract remaining quota and expiration
func (a *App) FetchSubscriptionInfo(subURL string) (*parser.SubscriptionFetchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return parser.FetchSubscriptionWithUserInfo(ctx, subURL)
}

// TestAllProfilesBatch concurrently tests latency for all profiles using a worker pool
func (a *App) TestAllProfilesBatch(category string) ([]core.BatchTestResult, error) {
	var profiles []config.Profile
	if category != "" && category != "all" {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	} else {
		profiles = a.store.GetProfiles()
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles to test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	results := core.BatchTestProfiles(ctx, profiles, 20, 2500*time.Millisecond, func(p core.BatchTestProgress) {
		if a.appCtx() != nil {
			runtime.EventsEmit(a.appCtx(), "ping:batch:progress", p)
		}
	})

	// Update store with new latencies in bulk (single file write)
	latencyMap := make(map[string]int)
	for _, r := range results {
		latencyMap[r.ProfileID] = r.LatencyMs
	}

	allProfiles := a.store.GetProfiles()
	var toUpdate []config.Profile
	for i := range allProfiles {
		if lat, ok := latencyMap[allProfiles[i].ID]; ok {
			allProfiles[i].LatencyMs = lat
			allProfiles[i].UpdatedAt = time.Now()
			toUpdate = append(toUpdate, allProfiles[i])
		}
	}
	if len(toUpdate) > 0 {
		_ = a.store.UpdateProfileBatch(toUpdate)
	}

	return results, nil
}

// RunDNSLeakDiagnostics performs real DNS leak probe through the active proxy
func (a *App) RunDNSLeakDiagnostics() (*core.DNSLeakReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	port := 10808
	st := a.engine.GetStatus()
	if st.State != core.StateConnected {
		// When not connected, test direct
		port = 0
	}
	return core.RunDNSLeakTest(ctx, port)
}

// GetConnectionHistory retrieves session logs
func (a *App) GetConnectionHistory() []config.ConnectionSession {
	if a.store == nil {
		return nil
	}
	return a.store.GetConnectionHistory()
}

// ClearConnectionHistory deletes session logs
func (a *App) ClearConnectionHistory() error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return a.store.ClearConnectionHistory()
}

// ExportFullBackup returns a JSON backup of all profiles and settings
func (a *App) ExportFullBackup() (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}
	return a.store.ExportFullBackupJSON()
}

// ImportFullBackup restores profiles and settings from a JSON bundle
func (a *App) ImportFullBackup(jsonStr string, overwrite bool) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	return a.store.ImportFullBackupJSON(jsonStr, overwrite)
}

// SendSystemNotification sends a native Windows Toast notification
func (a *App) SendSystemNotification(title, message string) {
	core.ShowWindowsNotification(title, message)
}

// CreateProxyChain combines multiple nodes into a single chained tunnel profile
func (a *App) CreateProxyChain(hopProfileIDs []string, chainName string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}

	var hops []config.Profile
	for _, id := range hopProfileIDs {
		p, err := a.store.GetProfile(id)
		if err != nil {
			return "", fmt.Errorf("profile %s not found: %w", id, err)
		}
		hops = append(hops, p)
	}

	settings := a.store.GetSettings()
	chainedProfile, err := parser.BuildMultiHopChain(hops, chainName, settings)
	if err != nil {
		return "", err
	}

	return a.store.AddProfile(*chainedProfile)
}

// StartMobileRemote launches local web controller for smartphone access
func (a *App) StartMobileRemote(port int) (string, error) {
	server := core.GetRemoteServer(a.engine, a.store)
	// Apply the persisted security policy before the listener is created, so
	// the LAN guard and TLS wrapping are decided up front.
	if a.store != nil {
		settings := a.store.GetSettings()
		server.SetSecureDefaults(settings.RemoteControllerLANOnly, settings.RemoteControllerTLS)
	}
	if err := server.Start(port); err != nil {
		return "", err
	}
	return server.GetURL(), nil
}

// MobileRemoteInfo holds status of the local smartphone web controller
type MobileRemoteInfo struct {
	Running bool   `json:"running"`
	URL     string `json:"url"`
	AuthURL string `json:"auth_url"`
	PIN     string `json:"pin"`
	// IdleTimeoutSec is the inactivity window after which the service stops
	// itself automatically, so the UI can warn the user.
	IdleTimeoutSec int `json:"idle_timeout_sec"`
	// TLSActive reports whether the controller is served over HTTPS with a
	// self-signed certificate. LANOnly reports whether non-RFC1918 peers are
	// refused at the listener. Both are surfaced so the UI can show the user
	// the real security posture rather than an aspirational claim.
	TLSActive bool `json:"tls_active"`
	LANOnly   bool `json:"lan_only"`
}

// GetMobileRemoteInfo returns the status, PIN and URL of the smartphone remote controller
func (a *App) GetMobileRemoteInfo() MobileRemoteInfo {
	if a.engine == nil || a.store == nil {
		return MobileRemoteInfo{Running: false, URL: "", AuthURL: "", PIN: ""}
	}
	server := core.GetRemoteServer(a.engine, a.store)
	transport := server.GetTransportSecurity()
	return MobileRemoteInfo{
		Running:        server.IsRunning(),
		URL:            server.GetURL(),
		AuthURL:        server.GetURLWithToken(),
		PIN:            server.GetAuthPIN(),
		IdleTimeoutSec: int(server.IdleTimeout().Seconds()),
		TLSActive:      transport.TLSActive,
		LANOnly:        transport.LANOnly,
	}
}

// GetProfileShareURI returns the raw URI or configuration content for QR code mobile export
func (a *App) GetProfileShareURI(profileID string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}
	profile, err := a.store.GetProfile(profileID)
	if err != nil {
		return "", err
	}
	if profile.RawURI != "" {
		return strings.TrimSpace(profile.RawURI), nil
	}
	if profile.ConfigContent != "" {
		return strings.TrimSpace(profile.ConfigContent), nil
	}
	return "", fmt.Errorf("no shareable content found for this profile")
}

// StopMobileRemote terminates mobile controller server
func (a *App) StopMobileRemote() {
	if a.engine != nil && a.store != nil {
		core.GetRemoteServer(a.engine, a.store).Stop()
	}
}

// PanicWipe immediately disconnects, restores system settings, flushes DNS, and
// purges logs. It reports the first genuine failure instead of always claiming
// success, so the emergency button can be trusted under pressure.
func (a *App) PanicWipe() error {
	var errs []string

	if a.engine != nil {
		if err := a.engine.Disconnect(); err != nil {
			errs = append(errs, "disconnect: "+err.Error())
		}
		a.engine.ClearLogs()
	}

	if err := sysproxy.ClearSystemProxy(); err != nil {
		errs = append(errs, "clear system proxy: "+err.Error())
	}

	// ForceCleanup, not Disable: this is the emergency button, so it must repair
	// the firewall even when the in-process bookkeeping believes nothing is set.
	if err := core.GetKillSwitch().ForceCleanup(); err != nil {
		errs = append(errs, "disable kill switch: "+err.Error())
	}

	if a.engine != nil && a.store != nil {
		core.GetRemoteServer(a.engine, a.store).Stop()
	}

	if repair := parser.RepairNetworkIssues(); !repair.Success {
		errs = append(errs, "network repair: "+repair.Message)
	}

	if len(errs) > 0 {
		return fmt.Errorf("panic wipe completed with errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// UpdateGeoDatabase downloads latest routing databases (geoip.dat and geosite.dat)
func (a *App) UpdateGeoDatabase() (map[string]string, error) {
	msg, err := core.UpdateGeoData()
	if err != nil {
		return map[string]string{
			"geoip.dat":   fmt.Sprintf("error: %v", err),
			"geosite.dat": fmt.Sprintf("error: %v", err),
		}, err
	}
	return map[string]string{
		"geoip.dat":   msg,
		"geosite.dat": msg,
	}, nil
}

// GenerateShareableSubscription encodes selected or all profiles into a standard base64 subscription format
func (a *App) GenerateShareableSubscription(category string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}

	var profiles []config.Profile
	if category == "" || category == "all" {
		profiles = a.store.GetProfiles()
	} else {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	}

	var links []string
	for _, p := range profiles {
		if p.RawURI != "" {
			links = append(links, strings.TrimSpace(p.RawURI))
		}
	}

	if len(links) == 0 {
		return "", fmt.Errorf("no valid shareable URI links found in selected category")
	}

	bundle := strings.Join(links, "\n")
	return base64.StdEncoding.EncodeToString([]byte(bundle)), nil
}

// UpdateCoreEngine downloads and updates a core binary (singbox, xray, openvpn, aether, psiphon)
func (a *App) UpdateCoreEngine(coreKey string) (string, error) {
	return core.UpdateCoreEngineWithProgress(coreKey, func(percent int, downloadedBytes int64, totalBytes int64, speedBytesPerSec int64, status string) {
		if a.appCtx() != nil {
			runtime.EventsEmit(a.appCtx(), "core:download_progress", map[string]interface{}{
				"key":        coreKey,
				"percent":    percent,
				"downloaded": downloadedBytes,
				"total":      totalBytes,
				"speed":      speedBytesPerSec,
				"status":     status,
			})
		}
	})
}

// UpdateGeoData updates GeoIP and GeoSite routing databases
func (a *App) UpdateGeoData() (string, error) {
	return core.UpdateGeoData()
}

// ConvertConfig converts between various config formats (Clash YAML, Sing-box JSON, Base64 sub, Raw links)
func (a *App) ConvertConfig(rawInput, targetFormat string) (string, error) {
	return parser.ConvertRawInputToFormat(rawInput, targetFormat)
}

// TestAllProfilesRealDelay tests real HTTP internet roundtrip latency across all profiles
func (a *App) TestAllProfilesRealDelay(category string) []core.BatchTestResult {
	if a.store == nil {
		return nil
	}

	var profiles []config.Profile
	if category == "" || category == "all" {
		profiles = a.store.GetProfiles()
	} else {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := core.BatchTestProfilesRealDelay(ctx, profiles, 20, 3*time.Second, func(p core.BatchTestProgress) {
		if a.appCtx() != nil {
			runtime.EventsEmit(a.appCtx(), "batch_test:real_progress", p)
		}
	})

	// Update latencies in store (batch)
	latencyMap2 := make(map[string]int)
	for _, r := range results {
		latencyMap2[r.ProfileID] = r.LatencyMs
	}
	allProfiles2 := a.store.GetProfiles()
	var toUpdate2 []config.Profile
	for i := range allProfiles2 {
		if lat, ok := latencyMap2[allProfiles2[i].ID]; ok {
			allProfiles2[i].LatencyMs = lat
			allProfiles2[i].UpdatedAt = time.Now()
			toUpdate2 = append(toUpdate2, allProfiles2[i])
		}
	}
	if len(toUpdate2) > 0 {
		_ = a.store.UpdateProfileBatch(toUpdate2)
	}

	return results
}

// ToggleAutoFailover enables or disables smart zero-downtime auto-reconnect
func (a *App) ToggleAutoFailover(enabled bool) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	settings := a.store.GetSettings()
	settings.AutoFailover = enabled
	settings.AutoFailoverEnabled = enabled
	return a.store.SaveSettings(settings)
}

// GetSubscriptionInfo fetches subscription metadata and user quota information
func (a *App) GetSubscriptionInfo(subUrl string) (*parser.SubscriptionUserInfo, error) {
	subUrl = strings.TrimSpace(subUrl)
	if subUrl == "" {
		return nil, fmt.Errorf("empty subscription URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := parser.FetchSubscriptionWithUserInfo(ctx, subUrl)
	if err != nil {
		return nil, err
	}
	return res.UserInfo, nil
}

// GetPsiphonRegions returns list of available countries and regions for Psiphon
func (a *App) GetPsiphonRegions() []string {
	out := make([]string, len(psiphonSupportedRegions))
	copy(out, psiphonSupportedRegions)
	return out
}

// GeneratePsiphonProfile returns a Psiphon emergency profile for the given region.
//
// It is idempotent: the profile is created on first use and reused afterwards,
// so repeatedly connecting to Psiphon no longer accumulates duplicate
// "Psiphon Emergency (XX)" entries in the profile list. The region is validated
// against the supported list so an arbitrary string can never be persisted.
// PsiphonOptions carries the advanced engine settings surfaced in the Psiphon
// page. Every field is honoured by the Psiphon adapter; before this struct
// existed the UI collected these values and silently discarded them, and the
// engine always ran with hard-coded SSH / 10808 / 10809.
type PsiphonOptions struct {
	TunnelProtocol string `json:"tunnel_protocol"` // "SSH", "OSSH", "FRONTED-MEEK-OSSH", "UNFRONTED-MEEK-OSSH", "QUIC"
	SocksPort      int    `json:"socks_port"`
	HTTPPort       int    `json:"http_port"`
	DisableSplit   bool   `json:"disable_split"`
}

// psiphonAllowedTunnelProtocols is the set of tunnel protocols the upstream
// psiphon-tunnel-core build accepts. Anything else is rejected rather than
// forwarded, so a bad value cannot make the engine fail at runtime.
var psiphonAllowedTunnelProtocols = map[string]bool{
	"SSH":                 true,
	"OSSH":                true,
	"FRONTED-MEEK-OSSH":   true,
	"UNFRONTED-MEEK-OSSH": true,
	"QUIC":                true,
}

// normalizePsiphonOptions validates and clamps user-supplied engine options.
func normalizePsiphonOptions(opts PsiphonOptions) PsiphonOptions {
	out := PsiphonOptions{
		TunnelProtocol: strings.ToUpper(strings.TrimSpace(opts.TunnelProtocol)),
		SocksPort:      opts.SocksPort,
		HTTPPort:       opts.HTTPPort,
		DisableSplit:   opts.DisableSplit,
	}
	if !psiphonAllowedTunnelProtocols[out.TunnelProtocol] {
		out.TunnelProtocol = "SSH"
	}
	// Ports must be in the unprivileged range and must not collide; otherwise
	// fall back to the historical defaults.
	if out.SocksPort < 1024 || out.SocksPort > 65535 {
		out.SocksPort = 10808
	}
	if out.HTTPPort < 1024 || out.HTTPPort > 65535 {
		out.HTTPPort = 10809
	}
	if out.SocksPort == out.HTTPPort {
		out.HTTPPort = out.SocksPort + 1
		if out.HTTPPort > 65535 {
			out.HTTPPort = out.SocksPort - 1
		}
	}
	return out
}

// GetPsiphonOptions returns the engine options currently persisted for the
// Psiphon profile, so the UI can display the real values instead of its own
// local defaults.
func (a *App) GetPsiphonOptions() PsiphonOptions {
	defaults := PsiphonOptions{TunnelProtocol: "SSH", SocksPort: 10808, HTTPPort: 10809}
	if a.store == nil {
		return defaults
	}
	for _, p := range a.store.GetProfilesByCategory(config.CategoryPsiphon) {
		content := strings.TrimSpace(p.ConfigContent)
		if !strings.HasPrefix(content, "{") {
			continue
		}
		var raw struct {
			TunnelProtocol string `json:"tunnel_protocol"`
			SocksPort      int    `json:"socks_port"`
			HttpPort       int    `json:"http_port"`
			DisableSplit   bool   `json:"disable_split"`
		}
		if err := json.Unmarshal([]byte(content), &raw); err != nil {
			continue
		}
		return normalizePsiphonOptions(PsiphonOptions{
			TunnelProtocol: raw.TunnelProtocol,
			SocksPort:      raw.SocksPort,
			HTTPPort:       raw.HttpPort,
			DisableSplit:   raw.DisableSplit,
		})
	}
	return defaults
}

// GeneratePsiphonProfile creates or refreshes the Psiphon profile for a region.
//
// It is idempotent: the profile is keyed by region, so repeated calls update the
// existing entry rather than accumulating duplicates (the original bug), and the
// advanced engine options are persisted into the profile config.
func (a *App) GeneratePsiphonProfile(region string) (*config.Profile, error) {
	return a.GeneratePsiphonProfileWithOptions(region, a.GetPsiphonOptions())
}

// GeneratePsiphonProfileWithOptions is GeneratePsiphonProfile with explicit
// engine options (used by the UI's advanced panel).
func (a *App) GeneratePsiphonProfileWithOptions(region string, opts PsiphonOptions) (*config.Profile, error) {
	if a.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}

	region = normalizePsiphonRegion(region)
	opts = normalizePsiphonOptions(opts)

	cfgObj := map[string]interface{}{
		"egress_region":   region,
		"socks_port":      opts.SocksPort,
		"http_port":       opts.HTTPPort,
		"tunnel_protocol": opts.TunnelProtocol,
		"disable_split":   opts.DisableSplit,
	}
	cfgBytes, _ := json.Marshal(cfgObj)

	profileName := "Psiphon Emergency Shield"
	if region != "AUTO" {
		profileName = fmt.Sprintf("Psiphon Emergency (%s)", region)
	}

	// Reuse an existing Psiphon profile for this region instead of creating a new one.
	for _, existing := range a.store.GetProfilesByCategory(config.CategoryPsiphon) {
		if existing.Name == profileName {
			existing.ConfigContent = string(cfgBytes)
			_ = a.store.UpdateProfile(existing)
			p := existing
			return &p, nil
		}
	}

	profile := &config.Profile{
		Name:          profileName,
		Protocol:      config.ProtocolPsiphon,
		Category:      config.CategoryPsiphon,
		ConfigContent: string(cfgBytes),
		Server:        "Psiphon Cloud Network",
		Group:         "Emergency Shield",
		Description:   "Emergency censorship circumvention multi-hop tunnel",
	}

	id, err := a.AddProfile(*profile)
	if err != nil {
		return nil, err
	}
	profile.ID = id
	return profile, nil
}

// psiphonSupportedRegions is the single source of truth for the regions the
// Psiphon engine accepts. The frontend reads it via GetPsiphonRegions so the UI
// list can never drift from the backend again.
var psiphonSupportedRegions = []string{
	"AUTO", "US", "DE", "GB", "NL", "CA", "FR", "JP", "SG", "AT", "CH", "IT", "ES", "PL", "SE", "RO", "BE", "IN",
}

// normalizePsiphonRegion upper-cases and validates a region, falling back to AUTO.
func normalizePsiphonRegion(region string) string {
	candidate := strings.ToUpper(strings.TrimSpace(region))
	if candidate == "" {
		return "AUTO"
	}
	for _, r := range psiphonSupportedRegions {
		if r == candidate {
			return candidate
		}
	}
	return "AUTO"
}

// ConnectPsiphon starts Psiphon emergency connection directly for a given region
func (a *App) ConnectPsiphon(region string) error {
	return a.ConnectPsiphonWithOptions(region, a.GetPsiphonOptions())
}

// ConnectPsiphonWithOptions starts Psiphon with explicit advanced engine
// options, so what the user set in the UI is what the engine actually runs.
func (a *App) ConnectPsiphonWithOptions(region string, opts PsiphonOptions) error {
	prof, err := a.GeneratePsiphonProfileWithOptions(region, opts)
	if err != nil {
		return err
	}
	return a.Connect(prof.ID)
}

// CheckServiceUnblock tests reachability of ChatGPT, YouTube, Spotify, Telegram, Discord, Steam, etc.
func (a *App) CheckServiceUnblock() ([]parser.ServiceUnblockResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	port := 0
	st := a.engine.GetStatus()
	if st.State == core.StateConnected {
		port = 10809
	}
	return parser.CheckServicesUnblock(ctx, port)
}

// ApplyGamingOptimizations applies Windows low-latency TCP/IP tweaks
func (a *App) ApplyGamingOptimizations() (*core.GamingOptimizationStatus, error) {
	return core.GetGamingOptimizer().ApplyGamingOptimizations()
}

// RevertGamingOptimizations restores Windows standard network stack configuration
func (a *App) RevertGamingOptimizations() (*core.GamingOptimizationStatus, error) {
	return core.GetGamingOptimizer().RevertGamingOptimizations()
}

// GetGamingOptimizationStatus checks if gaming network tweaks are active
func (a *App) GetGamingOptimizationStatus() core.GamingOptimizationStatus {
	return core.GetGamingOptimizer().GetStatus()
}

// ExportEncryptedBackup generates a plain or AES-256-GCM encrypted backup of all application data
func (a *App) ExportEncryptedBackup(password string) (string, error) {
	return config.ExportBackup(a.store, password)
}

// ImportEncryptedBackup imports and restores data from an encrypted or plain backup
func (a *App) ImportEncryptedBackup(content string, password string, overwrite bool) (*config.BackupSummary, error) {
	summary, err := config.ImportBackup(a.store, content, password, overwrite)
	if err != nil {
		return nil, err
	}
	a.broadcastStatus(a.engine.GetStatus())
	return summary, nil
}

// RunDiagnosticAudit scans system network stack, ports, binaries and drivers
func (a *App) RunDiagnosticAudit() *core.DiagnosticReport {
	return core.RunDiagnostics()
}

// RepairDiagnosticIssues automatically repairs common network, Wintun, proxy and stale core process issues
func (a *App) RepairDiagnosticIssues() *core.DiagnosticRepairResult {
	res := core.RepairAllIssues()
	a.broadcastStatus(a.engine.GetStatus())
	return res
}

// FlushWindowsDNS explicitly flushes the OS DNS cache and resets system proxy state
func (a *App) FlushWindowsDNS() error {
	return sysproxy.FlushDNSAndResetNetwork()
}

// GetSNIPoolList returns all candidate SNI domains (built-in + user custom)
func (a *App) GetSNIPoolList() []parser.SNIPoolItem {
	var custom []string
	if a.store != nil {
		custom = a.store.GetSettings().CustomSNIPool
	}
	return parser.GetCombinedSNIPool(custom)
}

// AddCustomSNI adds a user-defined SNI domain to the pool
func (a *App) AddCustomSNI(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	settings := a.store.GetSettings()
	for _, d := range settings.CustomSNIPool {
		if strings.EqualFold(d, domain) {
			return nil // already exists
		}
	}
	settings.CustomSNIPool = append(settings.CustomSNIPool, domain)
	return a.store.SaveSettings(settings)
}

// RemoveCustomSNI removes a custom SNI domain from the pool
func (a *App) RemoveCustomSNI(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	settings := a.store.GetSettings()
	var updated []string
	for _, d := range settings.CustomSNIPool {
		if !strings.EqualFold(d, domain) {
			updated = append(updated, d)
		}
	}
	settings.CustomSNIPool = updated
	return a.store.SaveSettings(settings)
}

// AuditSNIPool performs live TLS 1.3 handshake latency tests on all candidate SNIs (built-in + custom)
func (a *App) AuditSNIPool() []parser.SNIPoolItem {
	var custom []string
	if a.store != nil {
		custom = a.store.GetSettings().CustomSNIPool
	}
	return parser.AuditSNIPoolWithCustom(custom)
}

// OptimizeProfileSNI finds the fastest working SNI and patches the profile
func (a *App) OptimizeProfileSNI(profileID string) (*config.Profile, error) {
	p, err := a.store.GetProfile(profileID)
	if err != nil {
		return nil, fmt.Errorf("profile not found: %s (%w)", profileID, err)
	}

	newURI, newSNI, err := parser.OptimizeProfileSNI(p.RawURI)
	if err != nil {
		return &p, err
	}

	parsed, err := parser.ParseURI(newURI)
	if err != nil {
		return &p, err
	}

	p.RawURI = newURI
	p.ConfigContent = parsed.ConfigContent
	if err := a.store.UpdateProfile(p); err != nil {
		return &p, err
	}

	a.engine.AddLog("info", fmt.Sprintf("Optimized SNI for %s -> %s", p.Name, newSNI), "engine")
	return &p, nil
}

// MTUOptimizationResult is returned by RunAutoMTUOptimization so the UI can tell
// the difference between a discovered value and a value that was actually applied.
type MTUOptimizationResult struct {
	MTU       int    `json:"mtu"`
	Applied   bool   `json:"applied"`
	Supported bool   `json:"supported"`
	Message   string `json:"message"`
}

// RunAutoMTUOptimization probes the path to discover and apply the optimal Wintun MTU.
//
// The previous version discarded the apply error and always logged success, so the
// UI reported an MTU change even when the adapter was never touched. The result now
// states whether the value was really applied.
func (a *App) RunAutoMTUOptimization(targetHost string) MTUOptimizationResult {
	mtu := sysproc.DiscoverOptimalMTU(targetHost)

	// Ask the engine which adapter the active connection actually uses. The
	// engine may have had to avoid a ghost adapter by choosing a different name,
	// and applying the MTU to a name nothing is using would silently do nothing.
	adapterName := config.DefaultTunInterfaceName
	if a.engine != nil {
		adapterName = a.engine.TunInterfaceName()
	}

	applyErr := sysproc.ApplyWintunMTU(adapterName, mtu)
	if applyErr != nil {
		msg := fmt.Sprintf("Optimal MTU discovered: %d bytes, but it could not be applied: %v", mtu, applyErr)
		if a.engine != nil {
			a.engine.AddLog("warn", msg, "engine")
		}
		return MTUOptimizationResult{
			MTU:       mtu,
			Applied:   false,
			Supported: !errors.Is(applyErr, sysproc.ErrMTUUnsupported),
			Message:   msg,
		}
	}

	msg := fmt.Sprintf("Optimal MTU %d bytes applied to the tunnel adapter.", mtu)
	if a.engine != nil {
		a.engine.AddLog("info", msg, "engine")
	}
	return MTUOptimizationResult{
		MTU:       mtu,
		Applied:   true,
		Supported: true,
		Message:   msg,
	}
}

// CheckSubscriptionQuotas checks remaining data and expiration for a subscription URL
func (a *App) CheckSubscriptionQuotas(subURL string) (*core.QuotaAlertInfo, error) {
	qm := core.GetQuotaMonitor(a.store)
	return qm.CheckSubscriptionQuota(subURL)
}
