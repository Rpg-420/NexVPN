package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
	"nexvpn/internal/sysproxy"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartAutoQuarantineService is retained as an explicit no-op for API
// compatibility with older frontends.
//
// The service never quarantined anything: it only refreshed latency values,
// which the explicit "test all profiles" actions already do. Automatic latency
// probing every hour also produced false negatives on flaky networks, so it has
// been retired in favour of the user-initiated batch tests.
func (a *App) StartAutoQuarantineService() {
	if a.engine != nil {
		a.engine.AddLog("warn", "StartAutoQuarantineService is deprecated: hourly background probing was retired. Use the profile batch test instead.", "app")
	}
}

// ============================================================
// Profile Management
// ============================================================

func (a *App) GetProfiles() []config.Profile {
	if a.store == nil {
		return []config.Profile{}
	}
	return a.store.GetProfiles()
}

func (a *App) GetProfilesByCategory(category string) []config.Profile {
	if a.store == nil {
		return []config.Profile{}
	}
	return a.store.GetProfilesByCategory(config.ProfileCategory(category))
}

func (a *App) AddProfile(profile config.Profile) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}
	return a.store.AddProfile(profile)
}

func (a *App) UpdateProfile(profile config.Profile) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return a.store.UpdateProfile(profile)
}

func (a *App) DeleteProfile(id string) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return a.store.DeleteProfile(id)
}

func (a *App) DeleteAllProfiles() error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	// Fix #9: Use atomic ClearAllProfiles instead of looping DeleteProfile (N file writes -> 1)
	return a.store.ClearAllProfiles()
}

func (a *App) DeleteProfilesByCategory(category string) error {
	if a.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return a.store.DeleteProfilesByCategory(config.ProfileCategory(category))
}

func (a *App) ImportProfile(name string, protocol string, configContent string, group string, category string) (string, error) {
	cat := config.ProfileCategory(category)
	if cat == "" {
		cat = config.InferCategory(config.Protocol(protocol))
	}
	profile := config.Profile{
		Name:          name,
		Protocol:      config.Protocol(protocol),
		Category:      cat,
		ConfigContent: configContent,
		Group:         group,
		CreatedAt:     time.Now(),
	}
	return a.AddProfile(profile)
}

func (a *App) ImportURI(rawURI string, group string) (string, error) {
	rawURI = strings.TrimSpace(rawURI)
	lines := strings.Split(rawURI, "\n")
	if len(lines) > 1 {
		// Fix: batch all profiles into a single AddProfileBatch call (1 file write vs N)
		var batch []config.Profile
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			node, err := parser.ParseURI(line)
			if err == nil {
				if group != "" {
					node.Group = group
				}
				batch = append(batch, config.Profile{
					Name:          node.Name,
					Protocol:      node.Protocol,
					Category:      node.Category,
					ConfigContent: node.ConfigContent,
					RawURI:        node.RawURI,
					Server:        node.Server,
					Port:          node.Port,
					Group:         node.Group,
					CreatedAt:     time.Now(),
				})
			}
		}
		if len(batch) > 0 {
			ids, err := a.store.AddProfileBatch(batch)
			if err != nil {
				return "", err
			}
			return ids[len(ids)-1], nil
		}
	}

	node, err := parser.ParseURI(rawURI)
	if err != nil {
		return "", err
	}
	if group != "" {
		node.Group = group
	}

	profile := config.Profile{
		Name:          node.Name,
		Protocol:      node.Protocol,
		Category:      node.Category,
		ConfigContent: node.ConfigContent,
		RawURI:        node.RawURI,
		Server:        node.Server,
		Port:          node.Port,
		Group:         node.Group,
		CreatedAt:     time.Now(),
	}

	return a.AddProfile(profile)
}

func (a *App) ImportSubscription(subURL string, groupName string) (int, error) {
	nodes, err := parser.FetchSubscription(subURL)
	if err != nil {
		return 0, err
	}

	if groupName == "" {
		groupName = "Subscription"
	}

	var newProfiles []config.Profile
	for _, node := range nodes {
		profile := config.Profile{
			Name:             node.Name,
			Protocol:         node.Protocol,
			Category:         node.Category,
			ConfigContent:    node.ConfigContent,
			RawURI:           node.RawURI,
			Server:           node.Server,
			Port:             node.Port,
			Group:            groupName,
			SubscriptionURL:  subURL,
			SubscriptionName: groupName,
			CreatedAt:        time.Now(),
		}
		newProfiles = append(newProfiles, profile)
	}

	if err := a.store.ReplaceSubscriptionProfiles(subURL, newProfiles); err != nil {
		return 0, err
	}

	return len(newProfiles), nil
}

func (a *App) UpdateAllSubscriptions() (int, error) {
	profiles := a.store.GetProfiles()
	subMap := make(map[string]string)
	for _, p := range profiles {
		if p.SubscriptionURL != "" {
			name := p.SubscriptionName
			if name == "" {
				name = p.Group
			}
			subMap[p.SubscriptionURL] = name
		}
	}

	totalUpdated := 0
	for url, name := range subMap {
		if count, err := a.ImportSubscription(url, name); err == nil {
			totalUpdated += count
		}
	}

	return totalUpdated, nil
}

// DeleteGroup deletes all profiles belonging to a group or subscription
func (a *App) DeleteGroup(groupName string) error {
	return a.store.DeleteGroup(groupName)
}

// RenameGroup renames a group or subscription across all its profiles
func (a *App) RenameGroup(oldName string, newName string) error {
	return a.store.RenameGroup(oldName, newName)
}

// UpdateSubscriptionGroup updates profiles for a specific subscription group
func (a *App) UpdateSubscriptionGroup(groupName string) (int, error) {
	subURL := a.store.GetSubscriptionURLByGroup(groupName)
	if subURL == "" {
		return 0, fmt.Errorf("no subscription URL found for group %s", groupName)
	}
	return a.ImportSubscription(subURL, groupName)
}

// GenerateWarpAccount creates a 1-click Cloudflare WARP profile
func (a *App) GenerateWarpProfile(cleanIP string, port int) (*config.Profile, error) {
	if cleanIP == "" {
		cleanIP = "162.159.192.1"
	}
	if port <= 0 {
		port = 2408
	}

	profile, err := parser.GenerateCloudflareWarp(cleanIP, port)
	if err != nil {
		return nil, err
	}

	id, err := a.store.AddProfile(*profile)
	if err != nil {
		return nil, err
	}
	profile.ID = id
	return profile, nil
}

// GenerateWarpOnWarpProfile creates a 2-hop nested WARP-on-WARP profile
func (a *App) GenerateWarpOnWarpProfile(cleanIP string, port int, egressIP string, egressPort int) (*config.Profile, error) {
	profile, err := parser.GenerateWarpOnWarp(cleanIP, port, egressIP, egressPort, a.store.GetSettings())
	if err != nil {
		return nil, err
	}
	id, err := a.store.AddProfile(*profile)
	if err != nil {
		return nil, err
	}
	profile.ID = id
	return profile, nil
}

// DeduplicateProfiles removes duplicate nodes in a category
func (a *App) DeduplicateProfiles(category string) int {
	return a.store.DeduplicateProfiles(config.ProfileCategory(category))
}

// RemoveDeadProfiles removes all profiles with failed ping (latency <= 0)
func (a *App) RemoveDeadProfiles(category string) int {
	return a.store.RemoveDeadProfiles(config.ProfileCategory(category))
}

// ExportAllProfiles exports all profiles in a category as a newline-separated string
func (a *App) ExportAllProfiles(category string) string {
	return a.store.ExportAllProfiles(config.ProfileCategory(category))
}

// OpenTelegramProxy launches Telegram with the local SOCKS5 proxy URL.
//
// SECURITY: the URL is a hard-coded constant (no caller input), and we route
// it through the same scheme-validated launcher used for every other external
// link, removing the bespoke `cmd /c start` shell fallback entirely.
func (a *App) OpenTelegramProxy() error {
	const tgURL = "tg://socks?server=127.0.0.1&port=10808"
	return a.OpenExternalLink(tgURL)
}

// ImportOpenVPNProfile imports an OpenVPN .ovpn profile with optional auth credentials
func (a *App) ImportOpenVPNProfile(name string, ovpnContent string, username string, password string, group string) (string, error) {
	if name == "" {
		name = "OpenVPN Profile"
	}
	if group == "" {
		group = "OpenVPN"
	}

	profile := config.Profile{
		Name:          name,
		Protocol:      config.ProtocolOpenVPN,
		Category:      config.CategoryOpenVPN,
		ConfigContent: ovpnContent,
		Username:      username,
		Password:      password,
		Group:         group,
		CreatedAt:     time.Now(),
	}

	return a.store.AddProfile(profile)
}

// ImportWireGuardProfile imports a WireGuard .conf file
func (a *App) ImportWireGuardProfile(name string, confContent string, group string) (string, error) {
	if name == "" {
		name = "WireGuard Profile"
	}
	if group == "" {
		group = "WireGuard"
	}

	server := ""
	port := 51820
	outbound, host, err := parser.ParseWireGuardConf(confContent)
	if err == nil && outbound != nil {
		server = host
		if p, ok := outbound["server_port"].(int); ok && p > 0 {
			port = p
		}
	}

	profile := config.Profile{
		Name:          name,
		Protocol:      config.ProtocolWireGuard,
		Category:      config.CategoryWireGuardWarp,
		ConfigContent: confContent,
		Server:        server,
		Port:          port,
		Group:         group,
		CreatedAt:     time.Now(),
	}

	return a.store.AddProfile(profile)
}

func (a *App) PingProfile(id string) (int, error) {
	profile, err := a.store.GetProfile(id)
	if err != nil {
		return -1, err
	}

	server := profile.Server
	port := profile.Port

	// If server address is missing, extract it dynamically from config content
	if (server == "" || port <= 0) && profile.ConfigContent != "" && (profile.Protocol == config.ProtocolWireGuard || profile.Protocol == config.ProtocolWARP || profile.Category == config.CategoryWireGuardWarp) {
		_, host, err := parser.ParseWireGuardConf(profile.ConfigContent)
		if err == nil && host != "" {
			server = host
			profile.Server = host
		}
	}

	if server == "" {
		return -1, fmt.Errorf("profile has no server address")
	}

	latency := parser.SmartPing(server, port, profile.Protocol, 3*time.Second)
	profile.LatencyMs = latency
	_ = a.store.UpdateProfile(profile)

	return latency, nil
}

// pingAllProfilesConcurrency bounds how many profiles are probed at the same
// time. Without a cap, a subscription with hundreds of entries would spawn one
// goroutine (and one probe process/socket) per profile simultaneously.
const pingAllProfilesConcurrency = 16

// ProfilePingProgress is streamed while PingAllProfiles sweeps the list.
//
// The sweep returns only when every profile has been probed, and with a
// subscription-sized list at 3 s per probe over 16 workers that is tens of
// seconds during which the old UI showed nothing at all. Streaming the count
// turns an apparent hang into visible progress.
type ProfilePingProgress struct {
	TestedCount int    `json:"tested_count"`
	TotalCount  int    `json:"total_count"`
	Current     string `json:"current"`
	LatencyMs   int    `json:"latency_ms"`
	Alive       bool   `json:"alive"`
}

// PingAllProfiles probes every profile in a category and returns the updated
// list. Progress is emitted as `profiles:ping_progress`, and the finished list
// as `profiles:ping_complete`.
//
// The sweep can be stopped from the interface through CancelPingAll. A sweep of
// a large subscription takes minutes at three seconds per probe, and a button
// that cannot be taken back is a button users learn to avoid.
func (a *App) PingAllProfiles(category string) []config.Profile {
	a.pingAllCancel.Store(false)

	profiles := a.store.GetProfiles()

	cat := config.ProfileCategory(category)
	var targets []config.Profile
	for i := range profiles {
		// Match on the effective category (falling back to the protocol) so
		// legacy profiles with an empty Category are still pinged.
		if cat != "" && profiles[i].EffectiveCategory() != cat {
			continue
		}
		targets = append(targets, profiles[i])
	}

	// Bounded worker pool: keeps memory and socket/process usage flat regardless
	// of how many profiles the user has imported.
	workers := pingAllProfilesConcurrency
	if workers > len(targets) {
		workers = len(targets)
	}

	total := len(targets)
	var mu sync.Mutex
	var updatedProfiles []config.Profile
	var tested int
	var wg sync.WaitGroup

	emit := func(p ProfilePingProgress) {
		if ctx := a.appCtx(); ctx != nil {
			runtime.EventsEmit(ctx, "profiles:ping_progress", p)
		}
	}

	if workers > 0 {
		jobs := make(chan config.Profile)
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for p := range jobs {
					if a.pingAllCancel.Load() {
						return
					}

					server := p.Server
					port := p.Port
					if (server == "" || port <= 0) && p.ConfigContent != "" && (p.Protocol == config.ProtocolWireGuard || p.Protocol == config.ProtocolWARP || p.Category == config.CategoryWireGuardWarp) {
						_, host, err := parser.ParseWireGuardConf(p.ConfigContent)
						if err == nil && host != "" {
							server = host
							p.Server = host
						}
					}

					latency := -1
					if server != "" {
						latency = parser.SmartPing(server, port, p.Protocol, 3*time.Second)
						p.LatencyMs = latency
					}

					// `tested` counts every profile the sweep touched, including
					// the ones it had to skip. Counting only the probed ones made
					// the bar stop short of 100%, which reads as a failure rather
					// than a skip.
					mu.Lock()
					if server != "" {
						updatedProfiles = append(updatedProfiles, p)
					}
					tested++
					done := tested
					mu.Unlock()

					emit(ProfilePingProgress{
						TestedCount: done, TotalCount: total,
						Current:   p.Name,
						LatencyMs: latency,
						Alive:     latency > 0,
					})
				}
			}()
		}
		for _, p := range targets {
			// The feeder must stop too. Without this check it would block on an
			// unbuffered send the moment every worker returned, and the binding
			// would never return — a "stop" that hangs the app is worse than no
			// stop button at all.
			if a.pingAllCancel.Load() {
				break
			}
			jobs <- p
		}
		close(jobs)
		wg.Wait()
	}

	if len(updatedProfiles) > 0 {
		_ = a.store.UpdateProfileBatch(updatedProfiles)
	}

	var result []config.Profile
	if cat != "" {
		result = a.store.GetProfilesByCategory(cat)
	} else {
		result = a.store.GetProfiles()
	}

	if ctx := a.appCtx(); ctx != nil {
		runtime.EventsEmit(ctx, "profiles:ping_complete", result)
	}
	return result
}

// CancelPingAll stops a bulk ping sweep that is in progress.
//
// It is safe to call when nothing is running: the flag is reset at the start of
// every sweep, so a stray cancel cannot suppress the next one.
func (a *App) CancelPingAll() {
	a.pingAllCancel.Store(true)
}

func (a *App) RealPingActiveProfile() (int, error) {
	st := a.engine.GetStatus()
	if st.State != core.StateConnected {
		return -1, fmt.Errorf("VPN is not connected")
	}

	settings := a.store.GetSettings()
	testURL := settings.RealDelayUrl
	if testURL == "" {
		testURL = "http://cp.cloudflare.com/generate_204"
	}

	port := 0
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:10809", 200*time.Millisecond); err == nil {
		conn.Close()
		port = 10809
	} else if conn, err := net.DialTimeout("tcp", "127.0.0.1:10808", 200*time.Millisecond); err == nil {
		conn.Close()
		port = 10808
	}

	latency, err := parser.RealDelayPing(port, testURL, 4*time.Second)
	if err != nil {
		return -1, err
	}

	if st.ProfileID != "" {
		if profile, err := a.store.GetProfile(st.ProfileID); err == nil {
			profile.RealDelayMs = latency
			_ = a.store.UpdateProfile(profile)
		}
	}

	return latency, nil
}

func (a *App) LookupExternalIP() (*parser.IPInfo, error) {
	st := a.engine.GetStatus()
	port := 0
	if st.State == core.StateConnected {
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:10809", 200*time.Millisecond); err == nil {
			conn.Close()
			port = 10809
		} else if conn, err := net.DialTimeout("tcp", "127.0.0.1:10808", 200*time.Millisecond); err == nil {
			conn.Close()
			port = 10808
		}
	}
	return parser.LookupIPInfo(port)
}

func (a *App) ClearStaleProxy() {
	sysproxy.ClearSystemProxy()
}

func (a *App) GetGroups() []string {
	return a.store.GetGroupsByCategory("")
}

func (a *App) GetGroupsByCategory(category string) []string {
	return a.store.GetGroupsByCategory(config.ProfileCategory(category))
}

// OpenBrowserURL opens an external URL in the system default browser.
//
// SECURITY: the URL is caller-controlled, so it is routed through the same
// scheme allowlist as every other external launch. Invalid targets are
// silently dropped because the historical signature returns no error.
func (a *App) OpenBrowserURL(targetURL string) {
	if strings.TrimSpace(targetURL) == "" {
		return
	}
	if err := validateExternalLink(targetURL); err != nil {
		return
	}
	if ctx := a.appCtx(); ctx != nil {
		runtime.BrowserOpenURL(ctx, targetURL)
	}
}
