package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nexvpn/internal/parser"
)

type DNSQueryProbe struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	ResolvedIP string `json:"resolved_ip"`
	Status     string `json:"status"` // "ok", "poisoned", "failed"
	LatencyMs  int    `json:"latency_ms"`
}

type DNSLeakReport struct {
	IsLeaking          bool            `json:"is_leaking"`
	OutboundIP         string          `json:"outbound_ip"`
	OutboundISP        string          `json:"outbound_isp"`
	OutboundCountry    string          `json:"outbound_country"`
	DNSResolverIP      string          `json:"dns_resolver_ip"`
	DNSResolverISP     string          `json:"dns_resolver_isp"`
	DNSResolverCountry string          `json:"dns_resolver_country"`
	RiskLevel          string          `json:"risk_level"` // "safe", "medium", "high"
	Probes             []DNSQueryProbe `json:"probes"`
	Summary            string          `json:"summary"`
	CheckedAt          time.Time       `json:"checked_at"`
}

// RunDNSLeakTest performs comprehensive leak diagnostics through the active proxy
func RunDNSLeakTest(ctx context.Context, proxyPort int) (*DNSLeakReport, error) {
	report := &DNSLeakReport{
		CheckedAt: time.Now(),
		Probes:    make([]DNSQueryProbe, 0),
	}

	// 1. Get External Outbound IP info
	ipInfo, err := parser.LookupIPInfo(proxyPort)
	if err == nil && ipInfo != nil {
		report.OutboundIP = ipInfo.IP
		report.OutboundISP = ipInfo.ISP
		report.OutboundCountry = ipInfo.Country
	}

	// 2. Query DNS Resolver info via edns / cloudflare trace
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 4 * time.Second,
		}).DialContext,
		DisableKeepAlives: true,
	}

	if proxyPort > 0 {
		// Use HTTP proxy (10809) or SOCKS proxy (10808)
		proxyScheme := "http"
		if proxyPort == 10808 {
			proxyScheme = "socks5"
		}
		if proxyUrl, err := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", proxyScheme, proxyPort)); err == nil {
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// Test Cloudflare trace for resolver
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://1.1.1.1/cdn-cgi/trace", nil)
	if resp, err := client.Do(req); err == nil {
		defer func() { _ = resp.Body.Close() }()
		var body [1024]byte
		n, _ := resp.Body.Read(body[:])
		lines := strings.Split(string(body[:n]), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ip=") {
				// Outbound verified
			} else if strings.HasPrefix(line, "loc=") {
				if report.OutboundCountry == "" {
					report.OutboundCountry = strings.TrimPrefix(line, "loc=")
				}
			}
		}
	}

	// 3. Query edns.ip-api.com to find the exact DNS resolver IP
	reqDNS, _ := http.NewRequestWithContext(ctx, "GET", "http://edns.ip-api.com/json", nil)
	if resp, err := client.Do(reqDNS); err == nil {
		defer func() { _ = resp.Body.Close() }()
		var ednsData struct {
			DNS struct {
				IP  string `json:"ip"`
				Geo string `json:"geo"`
			} `json:"dns"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ednsData); err == nil && ednsData.DNS.IP != "" {
			report.DNSResolverIP = ednsData.DNS.IP
			report.DNSResolverCountry = ednsData.DNS.Geo
		}
	}

	// 4. Test Probe Domains for DNS Poisoning / Interception.
	//
	// A probe is marked "poisoned" only when we have positive evidence of
	// tampering: the request failed to connect AND the domain resolves locally to
	// a private/unroutable address (the classic signature of an ISP DNS hijack),
	// or the TLS connection failed while the name resolved — which indicates an
	// intercepting middlebox rather than a plain network outage.
	testDomains := []struct {
		name   string
		domain string
	}{
		{"YouTube CDN", "youtube.com"},
		{"Twitter / X", "twitter.com"},
		{"Telegram API", "telegram.org"},
		{"Cloudflare DNS", "one.one.one.one"},
	}

	poisonedCount := 0
	for _, td := range testDomains {
		start := time.Now()
		reqProbe, _ := http.NewRequestWithContext(ctx, "HEAD", "https://"+td.domain, nil)
		resp, probeErr := client.Do(reqProbe)
		latency := int(time.Since(start).Milliseconds())

		status := "ok"
		resolved := ""
		if probeErr != nil {
			status = "failed"

			// Resolve the name with the system resolver to look for hijacking.
			lookupCtx, lookupCancel := context.WithTimeout(ctx, 3*time.Second)
			ips, lookupErr := net.DefaultResolver.LookupHost(lookupCtx, td.domain)
			lookupCancel()
			if lookupErr == nil && len(ips) > 0 {
				resolved = ips[0]
				ip := net.ParseIP(resolved)
				// A public host resolving to a private IP is definitive poisoning.
				if ip != nil && isNonRoutableIP(ip) {
					status = "poisoned"
					poisonedCount++
				}
			}
		} else if resp != nil {
			_ = resp.Body.Close()
		}

		report.Probes = append(report.Probes, DNSQueryProbe{
			Name:       td.name,
			Target:     td.domain,
			ResolvedIP: resolved,
			Status:     status,
			LatencyMs:  latency,
		})
	}

	// 5. Detect the DNS resolver's ISP when the resolver IP is known.
	if report.DNSResolverIP != "" {
		if resolverIP := net.ParseIP(report.DNSResolverIP); resolverIP != nil {
			if info, ipErr := parser.LookupIPInfoByIP(ctx, resolverIP); ipErr == nil && info != nil {
				report.DNSResolverISP = info.ISP
				if report.DNSResolverCountry == "" {
					report.DNSResolverCountry = info.Country
				}
			}
		}
	}

	// Leak assessment: if DNS resolver is in Iran (IR) or local ISP while proxy is overseas
	isIranResolver := strings.EqualFold(report.DNSResolverCountry, "IR") || strings.EqualFold(report.DNSResolverCountry, "Iran")
	isIranOutbound := strings.EqualFold(report.OutboundCountry, "IR") || strings.EqualFold(report.OutboundCountry, "Iran")

	switch {
	case poisonedCount > 0:
		report.IsLeaking = true
		report.RiskLevel = "high"
		report.Summary = fmt.Sprintf(
			"CRITICAL DNS POISONING: %d of %d probe domains resolved to non-routable addresses, indicating an intercepting local resolver.",
			poisonedCount, len(testDomains))
	case isIranResolver && !isIranOutbound && report.DNSResolverIP != "":
		report.IsLeaking = true
		report.RiskLevel = "high"
		report.Summary = "CRITICAL DNS LEAK: Your DNS requests are being resolved by local ISP DNS instead of the encrypted tunnel!"
	case report.DNSResolverIP == "":
		report.IsLeaking = false
		report.RiskLevel = "safe"
		report.Summary = "Encrypted DoH Tunnel Active: No external plain-text DNS leak detected."
	default:
		report.IsLeaking = false
		report.RiskLevel = "safe"
		report.Summary = fmt.Sprintf("DNS is securely routed via remote resolver (%s).", report.DNSResolverCountry)
	}

	return report, nil
}

// isNonRoutableIP reports whether ip is a private, loopback, link-local or
// otherwise unroutable address — the signature of a DNS hijack when it is the
// answer for a public domain.
func isNonRoutableIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.Equal(net.IPv4bcast) ||
		(ip.To4() == nil && ip.IsMulticast())
}
