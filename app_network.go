package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
	sysruntime "runtime"
)

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
				if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || core.IsPrivate172(ipStr) {
					return ipStr, nil
				}
			}
		}
	}
	return "127.0.0.1", nil
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

	// `protocol` is the stable display label (also used as a React key, so it
	// must stay unique). `probe` is the config.Protocol handed to SmartPing and
	// MUST be one of the valid protocol constants — the previous code passed
	// variants such as "vless_reality"/"wireguard_warp" that match no constant,
	// so SmartPing fell through to a plain TCP ping and the WARP target never got
	// the WireGuard/UDP probe it is supposed to measure.
	targets := []struct {
		name     string
		protocol string
		probe    config.Protocol
		host     string
		port     int
		desc     string
	}{
		{"VLESS Reality (TLS 1.3)", "vless_reality", config.ProtocolXray, "162.159.192.1", 443, "Standard SNI / Reality anti-DPI tunnel"},
		{"Cloudflare WARP (WireGuard UDP)", "wireguard_warp", config.ProtocolWARP, "162.159.192.1", 2408, "Cloudflare clean UDP WireGuard port"},
		{"Hysteria 2 (QUIC / UDP 443)", "hysteria2", config.ProtocolHysteria2, "162.159.193.1", 443, "High-speed UDP congestion-controlled QUIC"},
		{"Fastly CDN (WebSocket / HTTPUpgrade)", "vmess_ws", config.ProtocolXray, "151.101.1.1", 443, "CDN-fronted WebSocket HTTP tunnel"},
		{"OpenVPN TCP (Fallback Tunnel)", "openvpn_tcp", config.ProtocolOpenVPN, "1.1.1.1", 443, "TCP encapsulation bypass mode"},
		{"Raw DNS (UDP 53 Hijack Test)", "dns_udp", config.ProtocolSingBox, "8.8.8.8", 53, "Local ISP DNS interception & poisoning probe"},
	}

	results := make([]RadarProtocolResult, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target struct {
			name     string
			protocol string
			probe    config.Protocol
			host     string
			port     int
			desc     string
		}) {
			defer wg.Done()

			// Measure real packet loss with a small, bounded sample of probes
			// instead of deriving it from latency. PacketLoss is the measured
			// percentage of probes that received no reply.
			const probeSamples = 4
			sent := 0
			received := 0
			totalLatency := 0
			bestLatency := -1

			for s := 0; s < probeSamples; s++ {
				sent++
				sample := parser.SmartPing(target.host, target.port, target.probe, 2500*time.Millisecond)
				if sample > 0 {
					received++
					totalLatency += sample
					if bestLatency < 0 || sample < bestLatency {
						bestLatency = sample
					}
				}
			}

			lat := -1
			if received > 0 {
				lat = totalLatency / received
			}
			loss := 0
			if sent > 0 {
				loss = ((sent - received) * 100) / sent
			}

			status := "excellent"
			switch {
			case received == 0:
				status = "blocked"
				loss = 100
			case loss > 0:
				status = "throttled"
			case lat > 300:
				status = "throttled"
			case lat > 150:
				status = "good"
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

// RunEmergencyNetworkRepair runs automated diagnostic and repair commands for networking
func (a *App) RunEmergencyNetworkRepair() (*EmergencyRepairReport, error) {
	var steps []struct {
		name string
		prog string
		args []string
	}

	switch sysruntime.GOOS {
	case "windows":
		steps = []struct {
			name string
			prog string
			args []string
		}{
			{"Flush Windows DNS Cache (ipconfig /flushdns)", "ipconfig", []string{"/flushdns"}},
			{"Reset Winsock Catalog & Network Sockets", "netsh", []string{"winsock", "reset"}},
			{"Reset Windows TCP/IP Protocol Stack", "netsh", []string{"int", "ip", "reset"}},
			{"Clear Routing Table & ARP Cache", "arp", []string{"-d", "*"}},
		}
	case "darwin":
		steps = []struct {
			name string
			prog string
			args []string
		}{
			{"Flush macOS DNS Cache (dscacheutil)", "dscacheutil", []string{"-flushcache"}},
			{"Reset DNS Responder (killall mDNSResponder)", "killall", []string{"-HUP", "mDNSResponder"}},
		}
	default: // linux
		steps = []struct {
			name string
			prog string
			args []string
		}{
			{"Flush Systemd DNS Cache (resolvectl)", "resolvectl", []string{"flush-caches"}},
			{"Clear ARP Neighbor Cache (ip neigh)", "ip", []string{"neigh", "flush", "all"}},
		}
	}

	reportSteps := make([]RepairStep, 0)
	failures := 0
	for _, s := range steps {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, s.prog, s.args...)
		cmd.SysProcAttr = sysproc.HideWindowAttr()
		out, err := cmd.CombinedOutput()
		cancel()

		status := "success"
		msg := strings.TrimSpace(string(out))
		if err != nil {
			// `arp -d *` exits non-zero when the ARP table is already empty, which is
			// a benign no-op rather than a repair failure. Everything else is a real error.
			benign := s.prog == "arp" && (strings.Contains(strings.ToLower(msg), "not found") ||
				strings.Contains(strings.ToLower(msg), "no arp") ||
				strings.Contains(strings.ToLower(msg), "not exist"))
			if benign {
				status = "skipped"
			} else {
				status = "error"
				failures++
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

	// Clear system proxy
	if err := sysproxy.ClearSystemProxy(); err != nil {
		failures++
		reportSteps = append(reportSteps, RepairStep{
			Name:    "Reset System Proxy Settings",
			Command: "SystemProxy: Clear",
			Status:  "error",
			Message: err.Error(),
		})
	} else {
		reportSteps = append(reportSteps, RepairStep{
			Name:    "Reset System Proxy Settings",
			Command: "SystemProxy: Clear",
			Status:  "success",
			Message: "System proxy settings cleaned and restored to default standard direct state.",
		})
	}

	// Overall status is derived from what actually happened — never hard-coded.
	var overall string
	switch {
	case failures == 0:
		overall = "success"
	case failures == len(reportSteps):
		overall = "failed"
	default:
		overall = "partial"
	}

	return &EmergencyRepairReport{
		OverallStatus: overall,
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
	// Source tells the UI where this entry came from so the "live" feed can be
	// labelled honestly: "remote" when fetched from the update manifest,
	// "builtin" when the curated fallback list is being served.
	Source string `json:"source"`
}

// cleanIPFeedManifestURL points at the community-maintained clean IP manifest.
// It is intentionally a plain JSON document (no redirects to third-party
// mirrors) so the app never executes or imports data from an untrusted host.
const cleanIPFeedManifestURL = "https://raw.githubusercontent.com/Rpg-420/NexVPN/main/data/clean_ips.json"

type cleanIPManifestEntry struct {
	CIDR        string `json:"cidr"`
	Region      string `json:"region"`
	Description string `json:"description"`
}

// builtinCleanIPRanges is the curated fallback used when the remote manifest is
// unreachable. Entries are marked with Source="builtin" so the UI can say so.
func builtinCleanIPRanges() []CleanIPRange {
	return []CleanIPRange{
		{"162.159.192.0/24", "IR-Clean (Cloudflare Edge)", "Best stability on Mobile ISPs (MCI & Irancell)", "builtin"},
		{"188.114.96.0/24", "IR-Clean (Cloudflare VIP)", "Ultra-low ping on Fixed ISPs (TCI & Shatel)", "builtin"},
		{"104.16.0.0/13", "Global CDN Anycast", "Optimized for VLESS Reality & WebSocket", "builtin"},
		{"172.64.0.0/13", "Cloudflare POP Frankfurt", "Optimized routing for gaming and streaming", "builtin"},
		{"198.41.128.0/17", "Cloudflare Tier-1", "High stability during peak bandwidth congestion", "builtin"},
		{"141.101.64.0/18", "Cloudflare Edge Subnet", "Lowest packet loss during peak hours", "builtin"},
	}
}

// FetchLiveCleanIPFeed fetches the community clean-IP manifest over the network.
// On any failure it degrades to the curated builtin list and marks every entry
// with Source="builtin", so the UI never presents static data as live data.
func (a *App) FetchLiveCleanIPFeed() ([]CleanIPRange, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", cleanIPFeedManifestURL, nil)
	if err == nil {
		req.Header.Set("Accept", "application/json")
		resp, reqErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if reqErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
				if readErr == nil {
					var remote []cleanIPManifestEntry
					if jsonErr := json.Unmarshal(body, &remote); jsonErr == nil {
						out := make([]CleanIPRange, 0, len(remote))
						for _, e := range remote {
							cidr := strings.TrimSpace(e.CIDR)
							// Only accept well-formed CIDRs, never arbitrary strings.
							if _, _, cidrErr := net.ParseCIDR(cidr); cidrErr != nil {
								continue
							}
							out = append(out, CleanIPRange{
								CIDR:        cidr,
								Region:      e.Region,
								Description: e.Description,
								Source:      "remote",
							})
						}
						if len(out) > 0 {
							return out, nil
						}
					}
				}
			}
		}
	}

	// Offline / malformed manifest: serve curated data, clearly labelled.
	return builtinCleanIPRanges(), nil
}
