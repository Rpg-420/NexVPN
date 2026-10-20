package dnsscan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// DNSServer represents a predefined DNS server for testing
type DNSServer struct {
	Name    string
	Address string // e.g., "8.8.8.8"
	IsDoH   bool
	DoHURL  string // e.g., "https://8.8.8.8/dns-query"
}

var PredefinedServers = []DNSServer{
	{Name: "Cloudflare", Address: "1.1.1.1", IsDoH: true, DoHURL: "https://1.1.1.1/dns-query"},
	{Name: "Google", Address: "8.8.8.8", IsDoH: true, DoHURL: "https://8.8.8.8/dns-query"},
	{Name: "Quad9", Address: "9.9.9.9", IsDoH: true, DoHURL: "https://9.9.9.9/dns-query"},
	{Name: "OpenDNS", Address: "208.67.222.222", IsDoH: false},
	{Name: "Shecan (Anti-Sanction IR)", Address: "178.22.122.100", IsDoH: false},
	{Name: "Electro (Anti-Sanction IR)", Address: "78.157.42.101", IsDoH: false},
	{Name: "Radar (Anti-Sanction IR)", Address: "10.202.10.10", IsDoH: false},
	{Name: "Level3", Address: "4.2.2.4", IsDoH: false},
}

type ScanResult struct {
	Server    DNSServer `json:"server"`
	Latency   int64     `json:"latency"` // Latency in milliseconds
	IsSuccess bool      `json:"is_success"`
	RawValue  string    `json:"raw_value"` // The value to save in config
}

// testUDPDNS measures the resolution time for a given DNS server
func testUDPDNS(serverAddress string, testDomain string, timeout time.Duration) (time.Duration, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}
			return d.DialContext(ctx, "udp", net.JoinHostPort(serverAddress, "53"))
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	ips, err := resolver.LookupHost(ctx, testDomain)
	if err != nil {
		return 0, err
	}

	// Ensure we got a valid response and not an empty array or a typical poisoned IP
	if len(ips) == 0 {
		return 0, fmt.Errorf("no IPs returned")
	}

	// Basic check against common Iranian poisoned IPs (e.g. 10.10.34.34)
	for _, ip := range ips {
		if strings.HasPrefix(ip, "10.10.3") {
			return 0, fmt.Errorf("poisoned DNS response detected: %s", ip)
		}
	}

	return time.Since(start), nil
}

// testDoHDNS measures resolution time using real RFC 8484 DNS over HTTPS query
func testDoHDNS(dohURL string, testDomain string, timeout time.Duration) (time.Duration, error) {
	var query []byte
	query = append(query, 0x12, 0x34) // ID
	query = append(query, 0x01, 0x00) // Flags: standard query, RD=1
	query = append(query, 0x00, 0x01) // QDCOUNT: 1
	query = append(query, 0x00, 0x00) // ANCOUNT: 0
	query = append(query, 0x00, 0x00) // NSCOUNT: 0
	query = append(query, 0x00, 0x00) // ARCOUNT: 0

	labels := strings.Split(testDomain, ".")
	for _, l := range labels {
		if len(l) == 0 {
			continue
		}
		query = append(query, byte(len(l)))
		query = append(query, []byte(l)...)
	}
	query = append(query, 0x00)       // Root label
	query = append(query, 0x00, 0x01) // Type: A (1)
	query = append(query, 0x00, 0x01) // Class: IN (1)

	// Certificate validation is intentionally left on: this app's threat model includes
	// active on-path DPI/MITM, and all predefined DoH endpoints (1.1.1.1, 8.8.8.8, 9.9.9.9)
	// present publicly trusted certs valid for their literal IP, so skipping verification
	// here would let a spoofed "secure" DNS response go undetected.
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", dohURL, bytes.NewReader(query))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("DoH HTTP status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil || len(body) < 12 {
		return 0, fmt.Errorf("invalid DoH response")
	}

	return time.Since(start), nil
}

// ScanAll runs tests against all predefined servers and returns them sorted by latency
func ScanAll(useDoHIfAvailable bool) []ScanResult {
	var results []ScanResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	testDomain := "cloudflare.com"
	timeout := 2 * time.Second

	for _, srv := range PredefinedServers {
		wg.Add(1)
		go func(s DNSServer) {
			defer wg.Done()

			var duration time.Duration
			var err error

			if useDoHIfAvailable && s.IsDoH && s.DoHURL != "" {
				duration, err = testDoHDNS(s.DoHURL, testDomain, timeout)
			} else {
				duration, err = testUDPDNS(s.Address, testDomain, timeout)
			}

			mu.Lock()
			defer mu.Unlock()

			isSuccess := err == nil
			latency := int64(timeout.Milliseconds())
			if isSuccess {
				latency = duration.Milliseconds()
			}

			// Generate RawValue for Sing-box config
			rawValue := s.Address
			if useDoHIfAvailable && s.IsDoH && s.DoHURL != "" {
				rawValue = s.DoHURL
			}

			results = append(results, ScanResult{
				Server:    s,
				Latency:   latency,
				IsSuccess: isSuccess,
				RawValue:  rawValue,
			})
		}(srv)
	}

	wg.Wait()

	// Sort results: Successful first, then by latency (lowest first)
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsSuccess && !results[j].IsSuccess {
			return true
		}
		if !results[i].IsSuccess && results[j].IsSuccess {
			return false
		}
		return results[i].Latency < results[j].Latency
	})

	return results
}
