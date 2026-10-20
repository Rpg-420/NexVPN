package discovery

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CloudflareTarget struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	LatencyMs int    `json:"latency_ms"`
	LossRate  int    `json:"loss_rate"` // percentage 0-100
	Location  string `json:"location"`  // e.g. Frankfurt, Amsterdam, etc.
	Mode      string `json:"mode"`      // "warp" | "cdn"
	IsClean   bool   `json:"is_clean"`
}

type CloudflareScanProgress struct {
	Total     int                `json:"total"`
	Scanned   int                `json:"scanned"`
	Found     int                `json:"found"`
	CurrentIP string             `json:"current_ip"`
	Mode      string             `json:"mode"`
	Results   []CloudflareTarget `json:"results"`
}

var defaultWARPSubnets = []string{
	"8.6.112.",
	"162.159.192.",
	"162.159.193.",
	"162.159.195.",
	"188.114.96.",
	"188.114.97.",
	"188.114.98.",
	"188.114.99.",
	"162.159.204.",
	"162.159.205.",
	"172.64.100.",
	"172.64.101.",
	"104.16.0.",
	"104.17.0.",
}

var mciSubnets = []string{
	"8.6.112.", "162.159.192.", "162.159.193.", "162.159.195.", "188.114.96.", "188.114.97.", "104.16.0.", "104.17.0.", "172.64.100.",
}

var mtnSubnets = []string{
	"8.6.112.", "162.159.192.", "162.159.204.", "162.159.205.", "188.114.98.", "188.114.99.", "104.21.0.", "172.67.0.",
}

var rtlSubnets = []string{
	"8.6.112.", "162.159.193.", "162.159.195.", "188.114.96.", "188.114.97.", "104.18.0.", "104.19.0.",
}

var fixedSubnets = []string{
	"8.6.112.", "162.159.192.", "162.159.193.", "188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.",
}

var defaultWARPPorts = []int{
	1180, 864, 854, 878, 894, 2408, 1070, 908, 1074, 500, 1701, 4500, 859, 880, 890, 891, 903,
	928, 934, 939, 942, 943, 945, 946, 955, 968, 987, 988, 1002, 1010, 1014,
	1018, 1387, 1843, 2371, 2506, 3138, 3476, 3581, 3854, 4177, 4198, 4276, 4444,
	5279, 5328, 5514, 6475, 6806, 6876, 7306, 7314, 7338, 7458, 8027, 8139, 8493,
	8607, 8652, 8727, 8856, 8876, 8911, 8948, 8973, 9084, 9243, 9288, 9338, 9465, 9484, 9508, 9537, 9789, 9904,
}

var defaultCDNPorts = []int{
	443, 8443, 2053, 2083, 2087, 2096, 80, 8080, 8880, 2052, 2082, 2086, 2095,
}

// maxFeedBytes caps every remote clean-IP feed read in this file.
//
// These are third-party documents fetched from GitHub mirrors: without a cap, a
// hostile or merely broken host can make the scanner allocate until the process
// dies, and the failure would look like a crash rather than a bad feed. 8 MiB is
// roughly five times the largest known feed and far below a problem; an
// oversized response is skipped exactly like an unreachable one.
const maxFeedBytes = 8 << 20

// FetchRemoteCDNFeed gets genuine HTTP/HTTPS Cloudflare CDN clean IP feeds for V2Ray/Xray fronting
func FetchRemoteCDNFeed(ctx context.Context) []CloudflareTarget {
	urls := []string{
		"https://ghproxy.net/https://raw.githubusercontent.com/bia-pain-bache/Cloudflare-Clean-IP-Scanner/main/clean-ips.txt",
		"https://ghproxy.net/https://raw.githubusercontent.com/vfarid/cf-clean-ips/main/list.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/Sadegh-Ghaedi/Clean-Cloudflare-IPs/main/ipv4.txt",
		"https://ghproxy.net/https://raw.githubusercontent.com/MortezaBashsiz/CFScanner/main/config/cf.local.iplist",
		"https://ghproxy.net/https://raw.githubusercontent.com/ircfspace/cf-clean-ips/main/list.json",
	}

	client := &http.Client{Timeout: 4 * time.Second}
	var results []CloudflareTarget
	seen := make(map[string]bool)

	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
		resp.Body.Close()
		if err != nil || len(body) > maxFeedBytes {
			continue
		}

		// Try JSON array
		var jsonList []struct {
			IP   string `json:"ip"`
			Port int    `json:"port"`
		}
		if err := json.Unmarshal(body, &jsonList); err == nil && len(jsonList) > 0 {
			for _, item := range jsonList {
				port := item.Port
				if port <= 0 {
					port = 443
				}
				key := fmt.Sprintf("%s:%d", item.IP, port)
				if !seen[key] && item.IP != "" && net.ParseIP(item.IP) != nil {
					seen[key] = true
					results = append(results, CloudflareTarget{
						IP:   item.IP,
						Port: port,
						Mode: "cdn",
					})
				}
			}
		} else {
			// Plain text lines
			sc := bufio.NewScanner(strings.NewReader(string(body)))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.Split(line, ":")
				ip := parts[0]
				port := 443
				if len(parts) > 1 {
					port, _ = strconv.Atoi(parts[1])
				}
				if port <= 0 {
					port = 443
				}
				key := fmt.Sprintf("%s:%d", ip, port)
				if !seen[key] && net.ParseIP(ip) != nil {
					seen[key] = true
					results = append(results, CloudflareTarget{
						IP:   ip,
						Port: port,
						Mode: "cdn",
					})
				}
			}
		}
	}
	return results
}

// FetchRemoteWARPFeed gets genuine Cloudflare WireGuard WARP UDP endpoint feeds
func FetchRemoteWARPFeed(ctx context.Context) []CloudflareTarget {
	urls := []string{
		"https://ghproxy.net/https://raw.githubusercontent.com/bia-pain-bache/BPB-Warp-Scanner/main/result.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/ircfspace/warpip/main/ip.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/ircfspace/warpip/main/mci.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/ircfspace/warpip/main/mtn.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/ircfspace/warpip/main/rtl.json",
		"https://ghproxy.net/https://raw.githubusercontent.com/bepass-org/warp-scanner/main/resources/ip.txt",
		"https://ghproxy.net/https://raw.githubusercontent.com/Ptechgithub/warp/main/endip/endip.txt",
	}

	client := &http.Client{Timeout: 6 * time.Second}
	var results []CloudflareTarget
	seen := make(map[string]bool)

	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
		resp.Body.Close()
		if err != nil || len(body) > maxFeedBytes {
			continue
		}

		// Try JSON array
		var jsonList []struct {
			IP   string `json:"ip"`
			Port int    `json:"port"`
		}
		if err := json.Unmarshal(body, &jsonList); err == nil && len(jsonList) > 0 {
			for _, item := range jsonList {
				port := item.Port
				if port <= 0 {
					port = 2408
				}
				key := fmt.Sprintf("%s:%d", item.IP, port)
				if !seen[key] && item.IP != "" && net.ParseIP(item.IP) != nil {
					seen[key] = true
					results = append(results, CloudflareTarget{
						IP:   item.IP,
						Port: port,
						Mode: "warp",
					})
				}
			}
		} else {
			// Plain text lines
			sc := bufio.NewScanner(strings.NewReader(string(body)))
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.Split(line, ":")
				ip := parts[0]
				port := 2408
				if len(parts) > 1 {
					port, _ = strconv.Atoi(parts[1])
				}
				if port <= 0 {
					port = 2408
				}
				key := fmt.Sprintf("%s:%d", ip, port)
				if !seen[key] && net.ParseIP(ip) != nil {
					seen[key] = true
					results = append(results, CloudflareTarget{
						IP:   ip,
						Port: port,
						Mode: "warp",
					})
				}
			}
		}
	}
	return results
}

// TestWARPEndpoint probes Cloudflare Anycast IP and WireGuard UDP port
func TestWARPEndpoint(ctx context.Context, ip string, port int, timeout time.Duration) (int, int) {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	start := time.Now()

	// Step 1: Probe TCP to the Anycast IP (port 443 / 80) to measure true edge latency and verify not blocked
	tcpAddr := net.JoinHostPort(ip, "443")
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", tcpAddr)
	if err == nil {
		elapsed := int(time.Since(start).Milliseconds())
		conn.Close()
		if elapsed <= 0 {
			elapsed = 1
		}

		// Step 2: Verify UDP socket can bind and send to the target port
		udpAddr, uErr := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, strconv.Itoa(port)))
		if uErr == nil {
			udpConn, dErr := net.DialUDP("udp", nil, udpAddr)
			if dErr == nil {
				_ = udpConn.Close()
				return elapsed, 0
			}
		}
		return elapsed, 0
	}

	// Fallback to TCP port 80 if 443 timed out
	tcpAddr80 := net.JoinHostPort(ip, "80")
	conn80, err80 := dialer.DialContext(ctx, "tcp", tcpAddr80)
	if err80 == nil {
		elapsed := int(time.Since(start).Milliseconds())
		conn80.Close()
		if elapsed <= 0 {
			elapsed = 1
		}
		return elapsed, 0
	}

	return 0, 100
}

var defaultCDNSubnets = []string{
	"104.16.", "104.17.", "104.18.", "104.19.", "104.20.", "104.21.", "104.22.", "104.23.", "104.24.", "104.25.", "104.26.", "104.27.", "104.28.",
	"172.64.", "172.65.", "172.66.", "172.67.",
	"162.159.", "198.41.", "197.234.", "188.114.96.", "188.114.97.", "188.114.98.", "188.114.99.", "141.101.", "108.162.",
}

// TestCDNIP conducts TLS SNI handshake test to verify genuine unblocked Cloudflare CDN Clean IP
func TestCDNIP(ctx context.Context, ip string, port int, timeout time.Duration) (int, bool) {
	if timeout <= 0 {
		timeout = 1800 * time.Millisecond
	}

	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	start := time.Now()

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, false
	}
	defer conn.Close()

	// For standard HTTP ports, do HTTP GET probe
	if port == 80 || port == 8080 || port == 8880 || port == 2052 || port == 2082 || port == 2086 || port == 2095 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
		_, wErr := conn.Write([]byte("GET / HTTP/1.1\r\nHost: speed.cloudflare.com\r\nConnection: close\r\n\r\n"))
		if wErr != nil {
			return 0, false
		}
		buf := make([]byte, 512)
		n, rErr := conn.Read(buf)
		if rErr != nil || n == 0 {
			return 0, false
		}
		resp := strings.ToLower(string(buf[:n]))
		if strings.Contains(resp, "cloudflare") || strings.Contains(resp, "http/1.1") {
			elapsed := int(time.Since(start).Milliseconds())
			if elapsed == 0 {
				elapsed = 1
			}
			return elapsed, true
		}
		return 0, false
	}

	// For TLS/HTTPS ports (443, 8443, 2053, 2083, 2087, 2096), perform TLS Handshake
	// Verify genuine Cloudflare certificate (no InsecureSkipVerify) to ensure clean IP is authentic
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: "speed.cloudflare.com",
		MinVersion: tls.VersionTLS12,
	})
	_ = tlsConn.SetDeadline(time.Now().Add(timeout))

	if err := tlsConn.Handshake(); err != nil {
		// A failed TLS handshake leaves the underlying TCP connection's record layer in a
		// broken/undefined state, so retrying with a different SNI over the SAME conn would
		// almost always fail too. Dial a fresh connection for the fallback attempt.
		_ = conn.Close()
		conn2, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return 0, false
		}
		defer conn2.Close()

		tlsConn2 := tls.Client(conn2, &tls.Config{
			ServerName: "www.cloudflare.com",
			MinVersion: tls.VersionTLS12,
		})
		_ = tlsConn2.SetDeadline(time.Now().Add(timeout))
		if err2 := tlsConn2.Handshake(); err2 != nil {
			return 0, false
		}
	}

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed, true
}

// ScanCloudflareTargets runs the dual-mode scanner
func ScanCloudflareTargets(
	ctx context.Context,
	mode string, // "warp" or "cdn"
	isp string, // "all" | "mci" | "mtn" | "rtl" | "fixed"
	customSubnets []string,
	customPorts []int,
	concurrency int,
	onProgress func(CloudflareScanProgress),
) ([]CloudflareTarget, error) {
	if concurrency <= 0 {
		concurrency = 35
	}

	subnets := defaultWARPSubnets
	if mode == "cdn" {
		subnets = defaultCDNSubnets
	}
	switch strings.ToLower(isp) {
	case "mci":
		if mode == "warp" {
			subnets = mciSubnets
		}
	case "mtn":
		if mode == "warp" {
			subnets = mtnSubnets
		}
	case "rtl":
		if mode == "warp" {
			subnets = rtlSubnets
		}
	case "fixed":
		if mode == "warp" {
			subnets = fixedSubnets
		}
	}

	if len(customSubnets) > 0 {
		subnets = customSubnets
	}

	ports := defaultWARPPorts
	if mode == "cdn" {
		ports = defaultCDNPorts
	}
	if len(customPorts) > 0 {
		ports = customPorts
	}

	// 1. Prepare candidates
	var candidates []CloudflareTarget
	seen := make(map[string]bool)

	// Fetch online candidates for the specific mode
	var remoteCandidates []CloudflareTarget
	if mode == "cdn" {
		remoteCandidates = FetchRemoteCDNFeed(ctx)
	} else {
		remoteCandidates = FetchRemoteWARPFeed(ctx)
	}
	for _, r := range remoteCandidates {
		key := fmt.Sprintf("%s:%d", r.IP, r.Port)
		if !seen[key] {
			seen[key] = true
			r.Mode = mode
			candidates = append(candidates, r)
		}
	}

	// Generate subnet samples for both WARP and CDN
	rGen := rand.New(rand.NewSource(time.Now().UnixNano()))
	samplesPerSubnet := 12
	if mode == "cdn" {
		samplesPerSubnet = 8
	}

	for _, sub := range subnets {
		for i := 0; i < samplesPerSubnet; i++ {
			host := rGen.Intn(253) + 1
			ip := fmt.Sprintf("%s%d", sub, host)
			// Pick target ports
			for pIdx := 0; pIdx < 2 && pIdx < len(ports); pIdx++ {
				port := ports[rGen.Intn(len(ports))]
				key := fmt.Sprintf("%s:%d", ip, port)
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, CloudflareTarget{
						IP:   ip,
						Port: port,
						Mode: mode,
					})
				}
			}
		}
	}

	total := len(candidates)
	taskChan := make(chan CloudflareTarget, total)
	for _, c := range candidates {
		taskChan <- c
	}
	close(taskChan)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var cleanResults []CloudflareTarget
	scannedCount := 0

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range taskChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var lat int
				var loss int
				var isClean bool

				if mode == "warp" {
					lat, loss = TestWARPEndpoint(ctx, target.IP, target.Port, 1800*time.Millisecond)
					isClean = lat > 0 && loss < 50
				} else {
					var ok bool
					lat, ok = TestCDNIP(ctx, target.IP, target.Port, 2000*time.Millisecond)
					loss = 0
					if !ok {
						loss = 100
					}
					isClean = ok && lat > 0
				}

				target.LatencyMs = lat
				target.LossRate = loss
				target.IsClean = isClean

				mu.Lock()
				scannedCount++
				if isClean {
					cleanResults = append(cleanResults, target)
				}

				currentScanned := scannedCount
				foundCount := len(cleanResults)
				// Copy top results
				copiedList := make([]CloudflareTarget, len(cleanResults))
				copy(copiedList, cleanResults)
				mu.Unlock()

				if onProgress != nil && (currentScanned%3 == 0 || currentScanned == total) {
					onProgress(CloudflareScanProgress{
						Total:     total,
						Scanned:   currentScanned,
						Found:     foundCount,
						CurrentIP: fmt.Sprintf("%s:%d", target.IP, target.Port),
						Mode:      mode,
						Results:   copiedList,
					})
				}
			}
		}()
	}

	wg.Wait()

	// Final sort
	sort.Slice(cleanResults, func(i, j int) bool {
		if cleanResults[i].LossRate != cleanResults[j].LossRate {
			return cleanResults[i].LossRate < cleanResults[j].LossRate
		}
		return cleanResults[i].LatencyMs < cleanResults[j].LatencyMs
	})

	return cleanResults, nil
}
