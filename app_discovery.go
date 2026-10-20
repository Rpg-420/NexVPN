package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"runtime/debug"
	"strconv"
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
		if a.appCtx() != nil {
			runtime.EventsEmit(a.appCtx(), "telegram:test_progress", p)
		}
	})

	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "telegram:test_complete", results)
	}

	return results
}

// allowedLinkSchemes is the closed allowlist of URL schemes that may ever be
// handed to the operating system's shell handler.
//
// SECURITY: previously any caller-supplied string was forwarded verbatim to
// ShellExecute / rundll32, which meant a malicious profile or an injected
// subscription payload could launch `file://`, `\\\\attacker\\share` (UNC),
// `javascript:`, or an arbitrary registered protocol handler. Restricting the
// scheme set to the handful we actually use closes that hole.
var allowedLinkSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"tg":     true, // tg://proxy?server=...&port=...
	"socks":  true, // some clients emit socks:// links
	"ss":     true, // shadowsocks:// share links
	"vless":  true,
	"vmess":  true,
	"trojan": true,
}

// ValidateExternalLink reports whether link is safe to hand to the OS shell.
// It is exported so the frontend and tests can pre-validate the same value.
func (a *App) ValidateExternalLink(link string) error {
	return validateExternalLink(link)
}

func validateExternalLink(link string) error {
	trimmed := strings.TrimSpace(link)
	if trimmed == "" {
		return fmt.Errorf("empty link")
	}
	// Reject control characters outright — they can be used to smuggle
	// arguments past naive parsers.
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("link contains control characters")
		}
	}
	// A UNC path such as \\host\share has no scheme at all; url.Parse would
	// treat it as an opaque path. Require an explicit scheme.
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid link: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		return fmt.Errorf("link has no scheme; refusing to open")
	}
	if !allowedLinkSchemes[scheme] {
		return fmt.Errorf("scheme %q is not allowed", scheme)
	}
	// For network schemes a host is mandatory; this rejects `http:`-only and
	// `file:`-style degenerate forms.
	if scheme != "tg" && (u.Host == "" && u.Opaque == "") {
		return fmt.Errorf("link %q has no host", trimmed)
	}
	return nil
}

// LaunchTelegramProxyLink opens the proxy link directly in Telegram Desktop
func (a *App) LaunchTelegramProxyLink(link string) error {
	link = strings.TrimSpace(link)
	if err := validateExternalLink(link); err != nil {
		return err
	}

	// Use Wails runtime BrowserOpenURL which executes OS ShellExecute safely without cmd.exe parameter splitting
	if a.appCtx() != nil {
		runtime.BrowserOpenURL(a.appCtx(), link)
		return nil
	}

	// Fallback using rundll32 url.dll,FileProtocolHandler (does not split on & unlike cmd.exe)
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	return cmd.Start()
}

// OpenExternalLink opens a validated external URL in the default handler.
// Any Go method that shells out a user-controlled URL must route through here.
func (a *App) OpenExternalLink(link string) error {
	link = strings.TrimSpace(link)
	if err := validateExternalLink(link); err != nil {
		return err
	}
	ctx := a.appCtx()
	if ctx == nil {
		return fmt.Errorf("application context is not ready")
	}
	runtime.BrowserOpenURL(ctx, link)
	return nil
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
		if a.appCtx() != nil {
			runtime.EventsEmit(a.appCtx(), "free_configs:test_progress", p)
		}
	})

	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "free_configs:test_complete", results)
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
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
			}
		}()

		results, err := discovery.ScanCloudflareTargets(ctx, mode, isp, customSubnets, customPorts, 35, func(p discovery.CloudflareScanProgress) {
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "cf_scanner:progress", p)
			}
		})

		// Terminal event is emitted even when the scan was cancelled, so the UI
		// can leave its "scanning" state after the user presses Stop.
		if a.appCtx() == nil {
			return
		}
		if err == nil {
			runtime.EventsEmit(a.appCtx(), "cf_scanner:complete", results)
		} else {
			runtime.EventsEmit(a.appCtx(), "cf_scanner:error", err.Error())
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

func updateURIHostPort(rawURI string, newIP string, newPort int) string {
	rawURI = strings.TrimSpace(rawURI)
	if rawURI == "" {
		return ""
	}
	fragment := ""
	if idx := strings.Index(rawURI, "#"); idx != -1 {
		fragment = rawURI[idx:]
		rawURI = rawURI[:idx]
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return rawURI + fragment
	}

	portStr := u.Port()
	if newPort > 0 {
		portStr = strconv.Itoa(newPort)
	}
	if portStr == "" {
		portStr = "443"
	}

	u.Host = net.JoinHostPort(newIP, portStr)
	return u.String() + fragment
}

func updateWgEndpointInConf(conf string, newIP string, newPort int) string {
	lines := strings.Split(conf, "\n")
	portStr := strconv.Itoa(newPort)
	if newPort <= 0 {
		portStr = "2408"
	}
	newEndpoint := fmt.Sprintf("Endpoint = %s:%s", newIP, portStr)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "endpoint") {
			lines[i] = newEndpoint
		}
	}
	return strings.Join(lines, "\n")
}

// ApplyCleanIPToWARPProfiles updates warp endpoint on all wireguard_warp profiles
func (a *App) ApplyCleanIPToWARPProfiles(ip string, port int) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}

	profiles := a.store.GetProfilesByCategory(config.CategoryWireGuardWarp)
	updatedCount := 0

	for _, p := range profiles {
		p.Server = ip
		if port > 0 {
			p.Port = port
		}
		p.WarpCleanIP = ip
		p.WarpEndpoint = fmt.Sprintf("%s:%d", ip, port)
		if p.RawURI != "" {
			p.RawURI = updateURIHostPort(p.RawURI, ip, port)
		}
		if p.ConfigContent != "" && strings.Contains(strings.ToLower(p.ConfigContent), "endpoint") {
			p.ConfigContent = updateWgEndpointInConf(p.ConfigContent, ip, port)
		}
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
		if p.RawURI != "" {
			p.RawURI = updateURIHostPort(p.RawURI, ip, port)
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
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
			}
		}()

		results, best, err := discovery.ScanFragmentSettings(ctx, nil, func(p discovery.FragmentScanProgress) {
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "frag_scanner:progress", p)
			}
		})

		if a.appCtx() == nil {
			return
		}
		if err == nil {
			runtime.EventsEmit(a.appCtx(), "frag_scanner:complete", map[string]interface{}{
				"results": results,
				"best":    best,
			})
		} else {
			runtime.EventsEmit(a.appCtx(), "frag_scanner:error", err.Error())
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

// GenerateSyncPIN returns a random 6-digit pairing PIN for nearby LAN profile
// sharing. A PIN is mandatory: without one the payload would have to be encrypted
// with a publicly known key, exposing proxy credentials to the whole LAN.
func (a *App) GenerateSyncPIN() string {
	return discovery.GenerateSyncPIN()
}

// BroadcastProfilesToLAN broadcasts profiles over local Wi-Fi network with optional pairing PIN
func (a *App) BroadcastProfilesToLAN(category string, deviceName string, pin string) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	var profiles []config.Profile
	if category == "" || category == "all" {
		profiles = a.store.GetProfiles()
	} else {
		profiles = a.store.GetProfilesByCategory(config.ProfileCategory(category))
	}
	return discovery.BroadcastProfilesToLAN(profiles, deviceName, pin)
}

// ListenForNearbyProfiles listens for incoming P2P Wi-Fi profile shares with optional pairing PIN
func (a *App) ListenForNearbyProfiles(timeoutSeconds int, pin string) (*discovery.SyncPayload, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 8
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	return discovery.ListenForNearbyProfiles(ctx, time.Duration(timeoutSeconds)*time.Second, pin)
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
