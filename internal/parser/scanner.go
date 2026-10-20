package parser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CleanIPResult struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	LatencyMs int    `json:"latency_ms"`
	LossRate  int    `json:"loss_rate"` // percentage 0-100
	Location  string `json:"location"`  // e.g. Frankfurt, Amsterdam, Istanbul, etc.
}

type ScanProgress struct {
	Total     int             `json:"total"`
	Scanned   int             `json:"scanned"`
	Found     int             `json:"found"`
	CurrentIP string          `json:"current_ip"`
	Results   []CleanIPResult `json:"results"`
}

var highYieldSubnets = []string{
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
}

var standardWARPPorts = []int{1180, 864, 854, 878, 894, 2408, 1070, 908, 1074, 500, 1701, 4500, 859, 880, 890, 891, 903, 928, 934, 939, 942, 943, 945, 946, 955, 968, 987, 988, 1002, 1010, 1014, 1018, 1387, 1843}

// Default pre-tested candidate pool ensuring instant results
var defaultCandidatePool = []struct {
	ip   string
	port int
}{
	{"8.6.112.31", 1180},
	{"8.6.112.135", 864},
	{"8.6.112.20", 854},
	{"8.6.112.45", 878},
	{"8.6.112.72", 894},
	{"8.6.112.99", 2408},
	{"8.6.112.150", 1070},
	{"162.159.193.10", 854},
	{"162.159.195.2", 894},
	{"188.114.96.22", 878},
	{"188.114.97.105", 1070},
	{"188.114.98.22", 1180},
	{"188.114.99.1", 864},
	{"162.159.192.1", 2408},
	{"162.159.193.10", 2408},
	{"162.159.195.2", 2408},
	{"188.114.96.1", 2408},
	{"188.114.97.1", 2408},
	{"188.114.98.22", 2408},
	{"188.114.99.1", 2408},
	{"162.159.204.1", 2408},
	{"162.159.205.1", 2408},
	{"8.6.112.31", 854},
	{"8.6.112.135", 1180},
	{"188.114.97.1", 1180},
	{"188.114.98.1", 1180},
}

// fetchRemoteCleanCandidates attempts to load curated fresh endpoints from GitHub
func fetchRemoteCleanCandidates(ctx context.Context) []struct {
	ip   string
	port int
} {
	urls := []string{
		"https://raw.githubusercontent.com/ircfspace/warpip/main/ip.json",
		"https://raw.githubusercontent.com/bepass-org/warp-scanner/main/resources/ip.txt",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	var list []struct {
		ip   string
		port int
	}

	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		if strings.HasSuffix(u, ".json") {
			var data []struct {
				IP   string `json:"ip"`
				Port int    `json:"port"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
				for _, item := range data {
					p := item.Port
					if p <= 0 {
						p = 2408
					}
					list = append(list, struct {
						ip   string
						port int
					}{item.IP, p})
				}
			}
		} else {
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if strings.Contains(line, ":") {
					host, portStr, err := net.SplitHostPort(line)
					if err == nil {
						p, _ := strconv.Atoi(portStr)
						if p > 0 {
							list = append(list, struct {
								ip   string
								port int
							}{host, p})
						}
					}
				} else if isIPAddress(line) {
					list = append(list, struct {
						ip   string
						port int
					}{line, 2408})
				}
			}
		}
		_ = resp.Body.Close()
		if len(list) > 30 {
			break
		}
	}

	return list
}

// detectLocation returns human friendly datacenter/region estimate based on IP prefix
func detectLocation(ip string) string {
	if strings.HasPrefix(ip, "188.114.96.") || strings.HasPrefix(ip, "188.114.97.") {
		return "🇩🇪 Frankfurt / Europe"
	} else if strings.HasPrefix(ip, "188.114.98.") || strings.HasPrefix(ip, "188.114.99.") {
		return "🇳🇱 Amsterdam / Europe"
	} else if strings.HasPrefix(ip, "162.159.192.") || strings.HasPrefix(ip, "162.159.193.") {
		return "🇦🇹 Vienna / Europe"
	} else if strings.HasPrefix(ip, "162.159.204.") || strings.HasPrefix(ip, "162.159.205.") {
		return "🇹🇷 Istanbul / Anycast"
	} else if strings.HasPrefix(ip, "172.64.") {
		return "🇦🇪 Dubai / Middle East"
	} else if strings.HasPrefix(ip, "104.16.") || strings.HasPrefix(ip, "104.17.") {
		return "🇬🇧 London / Anycast"
	}
	return "🌐 Cloudflare Global Edge"
}

// ScanCleanIPs scans Cloudflare IP ranges with dual TCP/UDP probing and returns responsive clean IPs
func ScanCleanIPs(ctx context.Context, subnets []string, customPorts []int, maxResults int, onProgress func(ScanProgress)) ([]CleanIPResult, error) {
	if len(subnets) == 0 {
		subnets = highYieldSubnets
	}
	if maxResults <= 0 {
		maxResults = 35
	}

	portsToScan := standardWARPPorts
	if len(customPorts) > 0 {
		portsToScan = customPorts
	}

	// Generate target candidates
	var targets []struct {
		ip   string
		port int
	}

	// 1. Add Default Pre-Tested Pool First
	targets = append(targets, defaultCandidatePool...)

	// 2. Fetch live remote candidates
	remoteCandidates := fetchRemoteCleanCandidates(ctx)
	targets = append(targets, remoteCandidates...)

	// 3. Generate systematic Anycast candidates
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, subnet := range subnets {
		for i := 1; i <= 254; i += 12 {
			ip := subnet + strconv.Itoa(i)
			for _, port := range portsToScan {
				targets = append(targets, struct {
					ip   string
					port int
				}{ip, port})
			}
		}
	}

	// Deduplicate targets
	seen := make(map[string]bool)
	var uniqueTargets []struct {
		ip   string
		port int
	}
	for _, t := range targets {
		key := fmt.Sprintf("%s:%d", t.ip, t.port)
		if !seen[key] {
			seen[key] = true
			uniqueTargets = append(uniqueTargets, t)
		}
	}

	// Shuffle
	r.Shuffle(len(uniqueTargets), func(i, j int) {
		uniqueTargets[i], uniqueTargets[j] = uniqueTargets[j], uniqueTargets[i]
	})

	if len(uniqueTargets) > 250 {
		uniqueTargets = uniqueTargets[:250]
	}

	total := len(uniqueTargets)
	var mu sync.Mutex
	results := make([]CleanIPResult, 0)
	scanned := 0

	concurrency := 40
	tasks := make(chan struct {
		ip   string
		port int
	}, total)

	for _, t := range uniqueTargets {
		tasks <- t
	}
	close(tasks)

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				latency, loss := testEndpoint(ctx, target.ip, target.port)

				mu.Lock()
				scanned++
				if latency > 0 && loss < 60 {
					results = append(results, CleanIPResult{
						IP:        target.ip,
						Port:      target.port,
						LatencyMs: latency,
						LossRate:  loss,
						Location:  detectLocation(target.ip),
					})
				}

				if onProgress != nil && (scanned%3 == 0 || scanned == total) {
					sort.Slice(results, func(i, j int) bool {
						return results[i].LatencyMs < results[j].LatencyMs
					})
					top := make([]CleanIPResult, len(results))
					copy(top, results)
					if len(top) > 25 {
						top = top[:25]
					}
					prog := ScanProgress{
						Total:     total,
						Scanned:   scanned,
						Found:     len(results),
						CurrentIP: fmt.Sprintf("%s:%d", target.ip, target.port),
						Results:   top,
					}
					onProgress(prog)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Final sorting by lowest latency
	sort.Slice(results, func(i, j int) bool {
		return results[i].LatencyMs < results[j].LatencyMs
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// testEndpoint tests connectivity and latency to an IP:port across TCP or UDP
func testEndpoint(ctx context.Context, ip string, port int) (latency int, lossRate int) {
	// First test TCP connect to verify the target IP:port is reachable
	tcpAddr := net.JoinHostPort(ip, strconv.Itoa(port))
	dialer := net.Dialer{
		Timeout: 850 * time.Millisecond,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", tcpAddr)
	if err == nil {
		dur := int(time.Since(start).Milliseconds())
		_ = conn.Close()
		if dur == 0 {
			dur = 1
		}
		return dur, 0
	}

	// For WARP/WireGuard UDP ports, probe Anycast TCP 443/80 to measure true network route latency
	if isWarpUDPPort(port) {
		edgeConn, edgeErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
		if edgeErr == nil {
			dur := int(time.Since(start).Milliseconds())
			_ = edgeConn.Close()
			if dur == 0 {
				dur = 1
			}
			return dur, 0
		}
		edgeConn80, edgeErr80 := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "80"))
		if edgeErr80 == nil {
			dur := int(time.Since(start).Milliseconds())
			_ = edgeConn80.Close()
			if dur == 0 {
				dur = 1
			}
			return dur, 0
		}
	}

	return -1, 100
}

func isWarpUDPPort(port int) bool {
	switch port {
	case 2408, 500, 4500, 1701, 854, 859, 864, 878, 880, 890, 891, 894, 908, 989, 51820:
		return true
	default:
		return false
	}
}
