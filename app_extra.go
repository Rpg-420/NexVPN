package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
	"nexvpn/internal/sysproxy"
)

var (
	scanMu      sync.Mutex
	scanCancel  context.CancelFunc
	speedMu     sync.Mutex
	speedCancel context.CancelFunc
)

// ShowWindow brings the window to the front
func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
}

// HideWindow hides the window to the system tray
func (a *App) HideWindow() {
	runtime.WindowHide(a.ctx)
}

// SetTitle sets the window title
func (a *App) SetTitle(title string) {
	runtime.WindowSetTitle(a.ctx, title)
}

func (a *App) broadcastStatus(status core.ConnectionStatus) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "status:changed", status)
	}
}

func (a *App) broadcastLog(entry core.LogEntry) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "log:entry", entry)
	}
}

func (a *App) broadcastStats(stats core.TrafficStats) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "stats:update", stats)
	}
}

func (a *App) broadcastHealth(health core.HealthStatus) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "health:update", health)
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
		runtime.BrowserOpenURL(a.ctx, "file://"+dataDir)
	}
}

func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"app_version": "1.3.1",
		"go_version":  "1.23.2",
		"platform":    "windows",
	}
}

func (a *App) CheckCoreBinaries() map[string]core.BinaryInfo {
	return core.CheckBinaries()
}

func (a *App) InstallCoreBinary(coreKey string) error {
	return core.DownloadAndInstallCore(coreKey)
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
		results, err := parser.ScanCleanIPs(ctx, customSubnets, customPorts, 35, func(p parser.ScanProgress) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "scanner:progress", p)
			}
		})
		if err == nil && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "scanner:complete", results)
		}
	}()
}

type BatchIPRequest struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// BatchGenerateWarpProfiles creates multiple WARP profiles for selected clean IPs
func (a *App) BatchGenerateWarpProfiles(targets []BatchIPRequest, useAmnezia bool) ([]config.Profile, error) {
	var created []config.Profile
	for i, t := range targets {
		if t.IP == "" {
			continue
		}
		p, err := parser.GenerateCloudflareWarp(t.IP, t.Port)
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

	port := 10808
	settings := a.store.GetSettings()
	if settings.ProxyMode == "system_proxy" {
		port = 10809
	}

	go func() {
		res, err := parser.RunSpeedTest(ctx, port, func(p parser.SpeedTestProgress) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "speedtest:progress", p)
			}
		})
		if err == nil && res != nil && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "speedtest:complete", *res)
		} else if err != nil && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "speedtest:error", err.Error())
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

// ApplyWarpLicenseKey updates settings with WARP+ license key and regenerates WARP account
func (a *App) ApplyWarpLicenseKey(licenseKey string) error {
	settings := a.store.GetSettings()
	settings.WarpLicenseKey = licenseKey
	settings.WarpAccountType = "plus"
	_ = a.store.SaveSettings(settings)

	// Regenerate Warp profile with clean IP
	cleanIP := settings.WarpCleanIP
	if cleanIP == "" {
		cleanIP = "162.159.192.1"
	}
	_, err := a.GenerateWarpProfile(cleanIP, settings.WarpPort)
	return err
}

// StartSubscriptionScheduler starts a background timer to periodically update subscriptions
func (a *App) StartSubscriptionScheduler() {
	go func() {
		settings := a.store.GetSettings()
		intervalHours := settings.SubscriptionAutoUpdateHours
		if intervalHours <= 0 {
			intervalHours = 12
		}

		ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			_, _ = a.UpdateAllSubscriptions()
		}
	}()
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
		best = &profiles[0]
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
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "ping:batch:progress", p)
		}
	})

	// Update store with new latencies in bulk
	latencyMap := make(map[string]int)
	for _, r := range results {
		latencyMap[r.ProfileID] = r.LatencyMs
	}

	allProfiles := a.store.GetProfiles()
	for i := range allProfiles {
		if lat, ok := latencyMap[allProfiles[i].ID]; ok {
			allProfiles[i].LatencyMs = lat
			allProfiles[i].UpdatedAt = time.Now()
			_ = a.store.UpdateProfile(allProfiles[i])
		}
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
	if err := server.Start(port); err != nil {
		return "", err
	}
	return server.GetURL(), nil
}

// StopMobileRemote terminates mobile controller server
func (a *App) StopMobileRemote() {
	if a.engine != nil && a.store != nil {
		core.GetRemoteServer(a.engine, a.store).Stop()
	}
}

// PanicWipe immediately disconnects, restores system settings, flushes DNS, and purges logs
func (a *App) PanicWipe() error {
	if a.engine != nil {
		_ = a.engine.Disconnect()
		a.engine.ClearLogs()
	}
	sysproxy.ClearSystemProxy()
	_ = core.GetKillSwitch().Disable()
	core.GetRemoteServer(a.engine, a.store).Stop()
	_ = parser.RepairNetworkIssues()
	return nil
}

// UpdateGeoDatabase downloads latest routing databases (geoip.dat and geosite.dat)
func (a *App) UpdateGeoDatabase() (map[string]string, error) {
	execDir, err := os.Executable()
	if err != nil {
		return nil, err
	}
	binDir := filepath.Join(filepath.Dir(execDir), "bin")
	_ = os.MkdirAll(binDir, 0755)

	targets := map[string]string{
		"geoip.dat":   "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
		"geosite.dat": "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
	}

	results := make(map[string]string)
	client := &http.Client{Timeout: 30 * time.Second}

	for filename, downloadURL := range targets {
		resp, err := client.Get(downloadURL)
		if err != nil {
			results[filename] = fmt.Sprintf("failed: %v", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			results[filename] = fmt.Sprintf("HTTP %d", resp.StatusCode)
			continue
		}

		targetPath := filepath.Join(binDir, filename)
		tmpPath := targetPath + ".download"

		outFile, createErr := os.Create(tmpPath)
		if createErr != nil {
			resp.Body.Close()
			results[filename] = fmt.Sprintf("create error: %v", createErr)
			continue
		}

		_, copyErr := io.Copy(outFile, resp.Body)
		outFile.Close()
		resp.Body.Close()

		if copyErr != nil {
			_ = os.Remove(tmpPath)
			results[filename] = fmt.Sprintf("download error: %v", copyErr)
			continue
		}

		_ = os.Remove(targetPath)
		_ = os.Rename(tmpPath, targetPath)
		results[filename] = "updated successfully"
	}

	return results, nil
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


