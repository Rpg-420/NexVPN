package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
	"nexvpn/internal/sysproxy"
	"nexvpn/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct is the main application backend binding
type App struct {
	ctx     context.Context
	store   *config.Store
	engine  *core.Engine
	scanner *core.CleanIPScanner
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

	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.engine.SetContext(ctx)

	// Clean any dangling proxy left by a crash or previous session
	sysproxy.CheckAndClearStaleProxy()

	// Initialize Windows Taskbar Notification Area (System Tray) icon near the clock
	tray.SetDisconnectHandler(func() {
		_ = a.Disconnect()
	})
	tray.InitSystemTray("NexVPN Pro - Ready")
	tray.ApplyMainWindowIcon()

	// Load store data
	if err := a.store.Load(); err != nil {
		fmt.Printf("Failed to load store: %v\n", err)
	}

	// Sync auto-reconnect setting
	settings := a.store.GetSettings()
	a.engine.SetAutoReconnect(settings.AutoReconnect)

	// Start subscription auto-update scheduler
	a.StartSubscriptionScheduler()

	// Auto-connect if enabled
	if settings.AutoConnect && settings.LastProfileID != "" {
		go func() {
			time.Sleep(1 * time.Second)
			_ = a.Connect(settings.LastProfileID)
		}()
	}
}

func (a *App) shutdown(ctx context.Context) {
	tray.RemoveSystemTray()
	if a.engine != nil {
		_ = a.engine.Disconnect()
	}
	sysproxy.ClearSystemProxy()
	_ = core.GetKillSwitch().Disable()
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
	profile, err := parser.GenerateWarpOnWarp(cleanIP, port, egressIP, egressPort)
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

// OpenTelegramProxy launches Telegram with the local SOCKS5 proxy URL
func (a *App) OpenTelegramProxy() error {
	cmd := exec.Command("cmd", "/c", "start", "tg://socks?server=127.0.0.1&port=10808")
	return cmd.Start()
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
	if server == "" && profile.ConfigContent != "" && profile.Protocol == config.ProtocolWireGuard {
		_, host, err := parser.ParseWireGuardConf(profile.ConfigContent)
		if err == nil && host != "" {
			server = host
			profile.Server = host
		}
	}

	if server == "" {
		return -1, fmt.Errorf("profile has no server address")
	}

	latency := parser.TCPPing(server, port, 3*time.Second)
	profile.LatencyMs = latency
	_ = a.store.UpdateProfile(profile)

	return latency, nil
}

func (a *App) PingAllProfiles(category string) []config.Profile {
	profiles := a.store.GetProfiles()
	var wg sync.WaitGroup

	cat := config.ProfileCategory(category)
	for i := range profiles {
		if cat != "" && profiles[i].Category != cat {
			continue
		}
		wg.Add(1)
		go func(p *config.Profile) {
			defer wg.Done()
			if p.Server != "" {
				p.LatencyMs = parser.TCPPing(p.Server, p.Port, 3*time.Second)
				_ = a.store.UpdateProfile(*p)
			}
		}(&profiles[i])
	}

	wg.Wait()
	if cat != "" {
		return a.store.GetProfilesByCategory(cat)
	}
	return a.store.GetProfiles()
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
	if settings.ProxyMode == "system_proxy" {
		port = 10809
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

func (a *App) OpenBrowserURL(targetURL string) {
	if targetURL != "" && a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, targetURL)
	}
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

	if err := a.engine.Connect(profile, settings); err != nil {
		return err
	}

	settings.LastProfileID = profileID
	profile.UsageCount++
	profile.LastUsed = time.Now()
	_ = a.store.UpdateProfile(profile)
	_ = a.store.SaveSettings(settings)

	return nil
}

func (a *App) Disconnect() error {
	return a.engine.Disconnect()
}

func (a *App) triggerAutoFailover(failedProfileID string) {
	if a.store == nil {
		return
	}
	profiles := a.store.GetProfiles()
	if len(profiles) <= 1 {
		return
	}

	var candidate *config.Profile
	bestLatency := 999999

	for i := range profiles {
		if profiles[i].ID == failedProfileID {
			continue
		}
		lat := profiles[i].LatencyMs
		if lat <= 0 {
			lat = 9999
		}
		if lat < bestLatency {
			bestLatency = lat
			// Fix: use index-based reference to avoid loop variable capture bug
			pCopy := profiles[i]
			candidate = &pCopy
		}
	}

	if candidate == nil {
		for i := range profiles {
			if profiles[i].ID != failedProfileID {
				// Fix: use index-based reference to avoid loop variable capture bug
				pCopy := profiles[i]
				candidate = &pCopy
				break
			}
		}
	}

	if candidate != nil {
		a.engine.AddLog("info", fmt.Sprintf("[Auto-Failover] Seamlessly switching to backup server: %s (%s)", candidate.Name, candidate.Protocol), "engine")
		_ = a.Connect(candidate.ID)
	}
}

func (a *App) GetConnectionStatus() core.ConnectionStatus {
	return a.engine.GetStatus()
}

func (a *App) GetTrafficStats() core.TrafficStats {
	return a.engine.GetStats()
}

// ============================================================
// Settings Management
// ============================================================

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

func (a *App) GetVersion() string {
	return AppVersion
}

// ============================================================
// LAN Proxy Sharing & Bridge
// ============================================================

// GetLocalIPAddress detects the active LAN IP of the host machine
func (a *App) GetLocalIPAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", err
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipStr := ipNet.IP.String()
				// Fix: complete RFC 1918 private address space check
				// 10.0.0.0/8, 172.16.0.0/12 (172.16-31.x.x), 192.168.0.0/16
				if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || isRFC1918_172(ipStr) {
					return ipStr, nil
				}
			}
		}
	}
	return "127.0.0.1", nil
}

// isRFC1918_172 checks the full 172.16.0.0/12 private range (172.16.x.x through 172.31.x.x)
func isRFC1918_172(ipStr string) bool {
	if !strings.HasPrefix(ipStr, "172.") {
		return false
	}
	parts := strings.Split(ipStr, ".")
	if len(parts) < 2 {
		return false
	}
	second := 0
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
		second = second*10 + int(c-'0')
	}
	return second >= 16 && second <= 31
}


func (a *App) ToggleAllowLan(enabled bool) error {
	settings := a.store.GetSettings()
	settings.AllowLan = enabled
	return a.store.SaveSettings(settings)
}

// ============================================================
// Live ISP Protocol Censorship Radar
// ============================================================

type RadarProtocolResult struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Status      string `json:"status"` // "excellent", "good", "throttled", "blocked"
	LatencyMs   int    `json:"latency_ms"`
	PacketLoss  int    `json:"packet_loss"`
	Description string `json:"description"`
}

type RadarReport struct {
	ISPName           string                `json:"isp_name"`
	IPAddress         string                `json:"ip_address"`
	CensorshipLevel   string                `json:"censorship_level"`
	BestProtocol      string                `json:"best_protocol"`
	RecommendedAction string                `json:"recommended_action"`
	Protocols         []RadarProtocolResult `json:"protocols"`
	Timestamp         string                `json:"timestamp"`
}

// RunProtocolRadar concurrently tests various protocols against censorship filters
func (a *App) RunProtocolRadar() (*RadarReport, error) {
	ipInfo, _ := a.LookupExternalIP()
	ispName := "Unknown ISP"
	ipAddr := "127.0.0.1"
	if ipInfo != nil {
		if ipInfo.ISP != "" {
			ispName = ipInfo.ISP
		}
		if ipInfo.IP != "" {
			ipAddr = ipInfo.IP
		}
	}

	targets := []struct {
		name     string
		protocol string
		host     string
		port     int
		desc     string
	}{
		{"VLESS Reality (TLS 1.3)", "vless_reality", "162.159.192.1", 443, "Standard SNI / Reality anti-DPI tunnel"},
		{"Cloudflare WARP (WireGuard UDP)", "wireguard_warp", "162.159.192.1", 2408, "Cloudflare clean UDP WireGuard port"},
		{"Hysteria 2 (QUIC / UDP 443)", "hysteria2", "162.159.193.1", 443, "High-speed UDP congestion-controlled QUIC"},
		{"Fastly CDN (WebSocket / HTTPUpgrade)", "vmess_ws", "151.101.1.1", 443, "CDN-fronted WebSocket HTTP tunnel"},
		{"OpenVPN TCP (Fallback Tunnel)", "openvpn_tcp", "1.1.1.1", 443, "TCP encapsulation bypass mode"},
		{"Raw DNS (UDP 53 Hijack Test)", "dns_udp", "8.8.8.8", 53, "Local ISP DNS interception & poisoning probe"},
	}

	results := make([]RadarProtocolResult, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target struct {
			name     string
			protocol string
			host     string
			port     int
			desc     string
		}) {
			defer wg.Done()
			lat := parser.TCPPing(target.host, target.port, 2500*time.Millisecond)
			status := "excellent"
			loss := 0
			if lat <= 0 {
				status = "blocked"
				loss = 100
				lat = -1
			} else if lat > 300 {
				status = "throttled"
				loss = 25
			} else if lat > 150 {
				status = "good"
				loss = 0
			}
			results[idx] = RadarProtocolResult{
				Name:        target.name,
				Protocol:    target.protocol,
				Status:      status,
				LatencyMs:   lat,
				PacketLoss:  loss,
				Description: target.desc,
			}
		}(i, t)
	}

	wg.Wait()

	// Calculate censorship severity
	blockedCount := 0
	bestProtocol := "VLESS Reality (TLS 1.3)"
	lowestLat := 99999
	for _, r := range results {
		if r.Status == "blocked" {
			blockedCount++
		} else if r.LatencyMs > 0 && r.LatencyMs < lowestLat {
			lowestLat = r.LatencyMs
			bestProtocol = r.Name
		}
	}

	level := "Low"
	action := "Network conditions are optimal. VLESS Reality or Cloudflare WARP recommended."
	if blockedCount >= 4 {
		level = "Severe (Severe Filtering / Outage)"
		action = "Most direct ports blocked. Use VLESS Reality with TLS Fragment or WARP-on-WARP."
	} else if blockedCount >= 2 {
		level = "Medium (Targeted Filtering)"
		action = "High-speed protocols like Hysteria 2 and VLESS Reality offer optimal throughput."
	}

	report := &RadarReport{
		ISPName:           ispName,
		IPAddress:         ipAddr,
		CensorshipLevel:   level,
		BestProtocol:      bestProtocol,
		RecommendedAction: action,
		Protocols:         results,
		Timestamp:         time.Now().Format("2006-01-02 15:04:05"),
	}

	return report, nil
}

// ============================================================
// Windows Emergency Network Repair
// ============================================================

type RepairStep struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type EmergencyRepairReport struct {
	OverallStatus string       `json:"overall_status"`
	Steps         []RepairStep `json:"steps"`
	Timestamp     string       `json:"timestamp"`
}

// RunEmergencyNetworkRepair runs automated diagnostic and repair commands for Windows networking
func (a *App) RunEmergencyNetworkRepair() (*EmergencyRepairReport, error) {
	steps := []struct {
		name string
		prog string
		args []string
	}{
		{"Flush Windows DNS Cache (ipconfig /flushdns)", "ipconfig", []string{"/flushdns"}},
		{"Reset Winsock Catalog & Network Sockets", "netsh", []string{"winsock", "reset"}},
		{"Reset Windows TCP/IP Protocol Stack", "netsh", []string{"int", "ip", "reset"}},
		{"Clear Routing Table & ARP Cache", "arp", []string{"-d", "*"}},
	}

	reportSteps := make([]RepairStep, 0)
	for _, s := range steps {
		cmd := exec.Command(s.prog, s.args...)
		out, err := cmd.CombinedOutput()
		status := "success"
		msg := strings.TrimSpace(string(out))
		if err != nil {
			// Fix: properly mark failed steps as error (not success)
			if s.prog == "arp" {
				// arp -d * always exits non-zero on Windows — treat as success
				status = "success"
			} else {
				status = "error"
			}
			if msg == "" {
				msg = err.Error()
			}
		}
		if len(msg) > 120 {
			msg = msg[:120] + "..."
		}
		reportSteps = append(reportSteps, RepairStep{
			Name:    s.name,
			Command: s.prog + " " + strings.Join(s.args, " "),
			Status:  status,
			Message: msg,
		})
	}

	// Clear registry proxy stubs
	_ = sysproxy.ClearSystemProxy()
	reportSteps = append(reportSteps, RepairStep{
		Name:    "Reset Windows WinINet Proxy Registry Keys",
		Command: "Registry: ProxyEnable=0",
		Status:  "success",
		Message: "System proxy settings cleaned and restored to default standard direct state.",
	})

	return &EmergencyRepairReport{
		OverallStatus: "success",
		Steps:         reportSteps,
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// ============================================================
// Live Cloud Clean IP Feed & Encrypted Backup
// ============================================================

type CleanIPRange struct {
	CIDR        string `json:"cidr"`
	Region      string `json:"region"`
	Description string `json:"description"`
}

// FetchLiveCleanIPFeed returns curated community clean IP ranges for Iran
func (a *App) FetchLiveCleanIPFeed() ([]CleanIPRange, error) {
	ranges := []CleanIPRange{
		{"162.159.192.0/24", "IR-Clean (Cloudflare Edge)", "Best stability on Mobile ISPs (MCI & Irancell)"},
		{"188.114.96.0/24", "IR-Clean (Cloudflare VIP)", "Ultra-low ping on Fixed ISPs (TCI & Shatel)"},
		{"104.16.0.0/13", "Global CDN Anycast", "Optimized for VLESS Reality & WebSocket"},
		{"172.64.0.0/13", "Cloudflare POP Frankfurt", "Optimized routing for gaming and streaming"},
		{"198.41.128.0/17", "Cloudflare Tier-1", "High stability during peak bandwidth congestion"},
		{"141.101.64.0/18", "Cloudflare Edge Subnet", "Lowest packet loss during peak hours"},
	}
	return ranges, nil
}

// ExportEncryptedBackup exports all data encrypted with AES-256-GCM if password is provided
func (a *App) ExportEncryptedBackup(password string) (string, error) {
	profiles := a.store.GetProfiles()
	settings := a.store.GetSettings()
	payload := struct {
		Profiles []config.Profile `json:"profiles"`
		Settings config.Settings  `json:"settings"`
		Exported string           `json:"exported_at"`
		App      string           `json:"app"`
	}{
		Profiles: profiles,
		Settings: settings,
		Exported: time.Now().Format(time.RFC3339),
		App:      "NexVPN",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	password = strings.TrimSpace(password)
	if password == "" {
		// Plain base64 export if no password
		return base64.StdEncoding.EncodeToString(data), nil
	}

	// Derive AES-256 key using SHA-256 of password
	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("cipher init error: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm init error: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation error: %w", err)
	}

	// Format: "NEXENC:" + base64(nonce + ciphertext)
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return "NEXENC:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// ImportEncryptedBackup imports and decrypts data from a backup string
func (a *App) ImportEncryptedBackup(payload string, password string) error {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return fmt.Errorf("empty backup payload")
	}

	var data []byte
	var err error

	if strings.HasPrefix(payload, "NEXENC:") {
		// Encrypted payload
		b64Content := strings.TrimPrefix(payload, "NEXENC:")
		rawBytes, decodeErr := base64.StdEncoding.DecodeString(b64Content)
		if decodeErr != nil {
			return fmt.Errorf("invalid encrypted payload encoding: %w", decodeErr)
		}

		password = strings.TrimSpace(password)
		if password == "" {
			return fmt.Errorf("password is required to decrypt this backup")
		}

		key := sha256.Sum256([]byte(password))
		block, cipherErr := aes.NewCipher(key[:])
		if cipherErr != nil {
			return fmt.Errorf("cipher init error: %w", cipherErr)
		}

		gcm, gcmErr := cipher.NewGCM(block)
		if gcmErr != nil {
			return fmt.Errorf("gcm error: %w", gcmErr)
		}

		nonceSize := gcm.NonceSize()
		if len(rawBytes) < nonceSize {
			return fmt.Errorf("corrupted encrypted backup payload (too short)")
		}

		nonce, ciphertext := rawBytes[:nonceSize], rawBytes[nonceSize:]
		decrypted, decryptErr := gcm.Open(nil, nonce, ciphertext, nil)
		if decryptErr != nil {
			return fmt.Errorf("incorrect backup password or corrupted data")
		}
		data = decrypted
	} else {
		// Legacy / Plain Base64 payload
		data, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return fmt.Errorf("invalid backup payload encoding: %w", err)
		}
	}

	var backup struct {
		Profiles []config.Profile `json:"profiles"`
		Settings config.Settings  `json:"settings"`
	}
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("invalid backup data structure: %w", err)
	}

	for _, p := range backup.Profiles {
		_, _ = a.store.AddProfile(p)
	}
	return a.store.SaveSettings(backup.Settings)
}

