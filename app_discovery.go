package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"nexvpn/internal/config"
	"nexvpn/internal/discovery"
	"nexvpn/internal/sysproc"
)

var (
	tgTestMu      sync.Mutex
	tgTestCancel  context.CancelFunc
	freeCfgMu     sync.Mutex
	freeCfgCancel context.CancelFunc
	cfScanMu      sync.Mutex
	cfScanCancel  context.CancelFunc
)

// FetchTelegramProxies retrieves telegram proxies from sources
func (a *App) FetchTelegramProxies(customSources []string) ([]discovery.TelegramProxy, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return discovery.FetchTelegramProxies(ctx, customSources)
}

// TestTelegramProxies tests latency and health of given telegram proxies
func (a *App) TestTelegramProxies(proxies []discovery.TelegramProxy) []discovery.TelegramProxy {
	tgTestMu.Lock()
	if tgTestCancel != nil {
		tgTestCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	tgTestCancel = cancel
	tgTestMu.Unlock()

	results := discovery.TestTelegramProxiesConcurrently(ctx, proxies, 30, 2500*time.Millisecond, func(p discovery.TelegramTestProgress) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "telegram:test_progress", p)
		}
	})

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "telegram:test_complete", results)
	}

	return results
}

// LaunchTelegramProxyLink opens the proxy link directly in Telegram Desktop
func (a *App) LaunchTelegramProxyLink(link string) error {
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Errorf("empty link")
	}

	// Use Wails runtime BrowserOpenURL which executes OS ShellExecute safely without cmd.exe parameter splitting
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, link)
		return nil
	}

	// Fallback using rundll32 url.dll,FileProtocolHandler (does not split on & unlike cmd.exe)
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	return cmd.Start()
}

// FetchFreeConfigs downloads and parses configs from aggregator sources
func (a *App) FetchFreeConfigs(customSources []string) ([]discovery.FreeConfigItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return discovery.FetchFreeConfigs(ctx, customSources)
}

// TestFreeConfigs tests latency of given free configs
func (a *App) TestFreeConfigs(configs []discovery.FreeConfigItem) []discovery.FreeConfigItem {
	freeCfgMu.Lock()
	if freeCfgCancel != nil {
		freeCfgCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	freeCfgCancel = cancel
	freeCfgMu.Unlock()

	results := discovery.TestFreeConfigsConcurrently(ctx, configs, 35, 2500*time.Millisecond, func(p discovery.FreeConfigTestProgress) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "free_configs:test_progress", p)
		}
	})

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "free_configs:test_complete", results)
	}

	return results
}

// ImportFreeConfigs imports selected free configs into the app profile store
func (a *App) ImportFreeConfigs(configs []discovery.FreeConfigItem, groupName string) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	if groupName == "" {
		groupName = "Free Community"
	}

	importedCount := 0
	for _, item := range configs {
		prof, err := discovery.ConvertFreeConfigToProfile(item, groupName)
		if err == nil {
			_, err = a.store.AddProfile(prof)
			if err == nil {
				importedCount++
			}
		}
	}

	return importedCount, nil
}

// StartCloudflareDiscoveryScan begins dual-mode scanning (warp or cdn) with ISP selection
func (a *App) StartCloudflareDiscoveryScan(mode string, isp string, customSubnets []string, customPorts []int) {
	cfScanMu.Lock()
	if cfScanCancel != nil {
		cfScanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cfScanCancel = cancel
	cfScanMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
			}
		}()

		results, err := discovery.ScanCloudflareTargets(ctx, mode, isp, customSubnets, customPorts, 35, func(p discovery.CloudflareScanProgress) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "cf_scanner:progress", p)
			}
		})

		if err == nil && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "cf_scanner:complete", results)
		}
	}()
}

// StopCloudflareDiscoveryScan stops the running Cloudflare scanner
func (a *App) StopCloudflareDiscoveryScan() {
	cfScanMu.Lock()
	if cfScanCancel != nil {
		cfScanCancel()
		cfScanCancel = nil
	}
	cfScanMu.Unlock()
}

// ApplyCleanIPToWARPProfiles updates warp endpoint on all wireguard_warp profiles
func (a *App) ApplyCleanIPToWARPProfiles(ip string, port int) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	profiles := a.store.GetProfilesByCategory(config.CategoryWireGuardWarp)
	updatedCount := 0

	for _, p := range profiles {
		p.WarpCleanIP = ip
		p.WarpEndpoint = fmt.Sprintf("%s:%d", ip, port)
		p.UpdatedAt = time.Now()
		if err := a.store.UpdateProfile(p); err == nil {
			updatedCount++
		}
	}

	// If no existing WARP profile, generate a fresh one
	if len(profiles) == 0 {
		if _, err := a.GenerateWarpProfile(ip, port); err == nil {
			updatedCount = 1
		}
	}

	// Also update in settings
	settings := a.store.GetSettings()
	settings.WarpCleanIP = ip
	settings.WarpPort = port
	_ = a.store.SaveSettings(settings)

	return updatedCount, nil
}

// ApplyCleanIPToV2RayProfiles updates server address with clean IP for V2Ray profiles
func (a *App) ApplyCleanIPToV2RayProfiles(ip string, port int, onlyCDN bool) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	profiles := a.store.GetProfilesByCategory(config.CategoryV2Ray)
	updatedCount := 0

	for _, p := range profiles {
		if onlyCDN {
			raw := strings.ToLower(p.RawURI)
			isCDN := strings.Contains(raw, "type=ws") || strings.Contains(raw, "type=grpc") ||
				strings.Contains(raw, "security=tls") || strings.Contains(raw, "security=reality") ||
				strings.Contains(raw, "sni=") || strings.Contains(raw, "host=")
			if !isCDN {
				continue
			}
		}

		p.Server = ip
		if port > 0 {
			p.Port = port
		}
		p.UpdatedAt = time.Now()
		if err := a.store.UpdateProfile(p); err == nil {
			updatedCount++
		}
	}

	return updatedCount, nil
}

var (
	fragScanMu     sync.Mutex
	fragScanCancel context.CancelFunc
)

// StartFragmentDiscoveryScan begins scanning and benchmarking optimal fragment sizes & sleeps
func (a *App) StartFragmentDiscoveryScan() {
	fragScanMu.Lock()
	if fragScanCancel != nil {
		fragScanCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	fragScanCancel = cancel
	fragScanMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
			}
		}()

		results, best, err := discovery.ScanFragmentSettings(ctx, nil, func(p discovery.FragmentScanProgress) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "frag_scanner:progress", p)
			}
		})

		if err == nil && a.ctx != nil {
			runtime.EventsEmit(a.ctx, "frag_scanner:complete", map[string]interface{}{
				"results": results,
				"best":    best,
			})
		}
	}()
}

// StopFragmentDiscoveryScan stops the active fragment scanner
func (a *App) StopFragmentDiscoveryScan() {
	fragScanMu.Lock()
	if fragScanCancel != nil {
		fragScanCancel()
		fragScanCancel = nil
	}
	fragScanMu.Unlock()
}

// ApplyOptimalFragment applies optimal fragment setting to application settings
func (a *App) ApplyOptimalFragment(size string, sleep string) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}

	settings := a.store.GetSettings()
	settings.FragmentEnabled = true
	settings.FragmentSize = strings.TrimSpace(size)
	settings.FragmentSleep = strings.TrimSpace(sleep)

	return a.store.SaveSettings(settings)
}

// DetectCurrentISP detects operator and provides tailor-made protocol recommendations
func (a *App) DetectCurrentISP() (*discovery.ISPRecommendation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return discovery.DetectCurrentISP(ctx)
}

// BroadcastProfilesToLAN broadcasts profiles over local Wi-Fi network
func (a *App) BroadcastProfilesToLAN(category string, deviceName string) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	var profiles []config.Profile
	if category == "" || category == "all" {
		profiles = a.store.GetProfiles()
	} else {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	}
	return discovery.BroadcastProfilesToLAN(profiles, deviceName)
}

// ListenForNearbyProfiles listens for incoming P2P Wi-Fi profile shares
func (a *App) ListenForNearbyProfiles(timeoutSeconds int) (*discovery.SyncPayload, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 8
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	return discovery.ListenForNearbyProfiles(ctx, time.Duration(timeoutSeconds)*time.Second)
}

// ImportNearbyProfiles saves received P2P profiles into local store
func (a *App) ImportNearbyProfiles(profiles []config.Profile, groupName string) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if groupName == "" {
		groupName = "Nearby Wi-Fi Share"
	}
	count := 0
	for _, p := range profiles {
		p.Group = groupName
		p.ID = "" // Generate fresh ID
		if _, err := a.store.AddProfile(p); err == nil {
			count++
		}
	}
	return count, nil
}

