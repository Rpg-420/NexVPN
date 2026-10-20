package discovery

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FragmentCandidate struct {
	ID          string `json:"id"`
	SizeRange   string `json:"size_range"`  // e.g. "10-30", "50-100", "1-5", "100-200"
	SleepRange  string `json:"sleep_range"` // e.g. "5-15", "10-20", "20-50", "1-5"
	MinSize     int    `json:"min_size"`
	MaxSize     int    `json:"max_size"`
	MinSleepMs  int    `json:"min_sleep_ms"`
	MaxSleepMs  int    `json:"max_sleep_ms"`
	SuccessRate int    `json:"success_rate"` // percentage 0-100
	AvgLatency  int    `json:"avg_latency"`  // ms
	Status      string `json:"status"`       // "optimal", "passed", "failed"
	Score       int    `json:"score"`        // 0-100
	Recommended bool   `json:"recommended"`
}

type FragmentScanProgress struct {
	Total       int                 `json:"total"`
	Scanned     int                 `json:"scanned"`
	Current     string              `json:"current"`
	BestSetting *FragmentCandidate  `json:"best_setting,omitempty"`
	Results     []FragmentCandidate `json:"results"`
}

var defaultFragmentCandidates = []struct {
	size  string
	sleep string
}{
	{"10-30", "10-20"},  // MCI Aggressive
	{"50-100", "5-15"},  // Irancell Stealth
	{"100-200", "5-10"}, // Fixed / ADSL Mild
	{"1-5", "15-30"},    // Extreme DPI Bypass
	{"20-50", "10-20"},  // Balanced Medium
	{"30-60", "5-15"},   // Fast Mobile
	{"80-150", "5-10"},  // Fast Stable
	{"10-20", "20-40"},  // Heavy Censorship
	{"50-80", "10-25"},  // Standard TLS
	{"150-300", "2-8"},  // High Bandwidth
}

type probeTarget struct {
	addr string
	sni  string
}

var testTargetProbes = []probeTarget{
	{addr: "104.16.132.229:443", sni: "www.cloudflare.com"},
	{addr: "172.64.155.209:443", sni: "cloudflare.com"},
	{addr: "142.250.185.206:443", sni: "www.google.com"},
	{addr: "151.101.65.140:443", sni: "www.reddit.com"},
	{addr: "162.159.130.233:443", sni: "discord.com"},
}

// Generate a valid standard TLS 1.2/1.3 ClientHello payload with SNI
func buildClientHello(sni string) []byte {
	sniBytes := []byte(sni)
	sniLen := len(sniBytes)

	// Extension: Server Name Indication (SNI)
	extSNI := []byte{
		0x00, 0x00, // Extension: Server Name (0)
		byte((sniLen + 5) >> 8), byte(sniLen + 5), // Ext length
		byte((sniLen + 3) >> 8), byte(sniLen + 3), // ServerNameList length
		0x00, // NameType: host_name
		byte(sniLen >> 8), byte(sniLen),
	}
	extSNI = append(extSNI, sniBytes...)

	// Extension: Supported Versions (TLS 1.3 & 1.2)
	extVersions := []byte{
		0x00, 0x2b, // Extension: supported_versions
		0x00, 0x03, // Ext length: 3
		0x02,       // Supported versions list length: 2
		0x03, 0x04, // TLS 1.3 (0x0304)
	}

	allExts := append(extSNI, extVersions...)
	extsLen := len(allExts)

	buf := []byte{
		0x16,       // ContentType: Handshake (22)
		0x03, 0x01, // Version: TLS 1.0 (Record header)
		0x00, 0x00, // Record Length (placeholder at [3..4])
		0x01,             // HandshakeType: ClientHello (1)
		0x00, 0x00, 0x00, // Handshake Body Length (placeholder at [6..8])
		0x03, 0x03, // Client Version: TLS 1.2
		// 32-byte Random
		0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0,
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x00,       // Session ID Length: 0
		0x00, 0x08, // Cipher Suites Length: 8
		0x13, 0x01, // TLS_AES_128_GCM_SHA256 (TLS 1.3)
		0x13, 0x02, // TLS_AES_256_GCM_SHA384 (TLS 1.3)
		0xc0, 0x2f, // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 (TLS 1.2)
		0xc0, 0x30, // TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 (TLS 1.2)
		0x01, 0x00, // Compression methods: 1 byte (null: 0x00)
		byte(extsLen >> 8), byte(extsLen), // Extensions total length
	}

	buf = append(buf, allExts...)

	// Update lengths
	handshakeLen := len(buf) - 5
	buf[3] = byte(handshakeLen >> 8)
	buf[4] = byte(handshakeLen)

	bodyLen := len(buf) - 9
	buf[6] = byte(bodyLen >> 16)
	buf[7] = byte(bodyLen >> 8)
	buf[8] = byte(bodyLen)

	return buf
}

// Parse range strings like "10-30" into (10, 30)
func parseRange(rStr string, defMin, defMax int) (int, int) {
	parts := strings.Split(rStr, "-")
	if len(parts) == 2 {
		minV, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		maxV, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil && minV > 0 && maxV >= minV {
			return minV, maxV
		}
	}
	return defMin, defMax
}

// Random int in [min, max]
func randRange(minV, maxV int) int {
	if maxV <= minV {
		return minV
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(maxV-minV+1)))
	return minV + int(n.Int64())
}

// TestSingleFragmentCandidate executes a real fragment simulation test on network targets
func TestSingleFragmentCandidate(
	ctx context.Context,
	candidate FragmentCandidate,
	timeout time.Duration,
) FragmentCandidate {
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	minSize, maxSize := parseRange(candidate.SizeRange, 10, 30)
	minSleep, maxSleep := parseRange(candidate.SleepRange, 5, 20)

	candidate.MinSize = minSize
	candidate.MaxSize = maxSize
	candidate.MinSleepMs = minSleep
	candidate.MaxSleepMs = maxSleep

	successCount := 0
	totalLatency := 0
	trials := len(testTargetProbes)

	dialer := &net.Dialer{Timeout: timeout}

	for _, target := range testTargetProbes {
		select {
		case <-ctx.Done():
			return candidate
		default:
		}

		clientHello := buildClientHello(target.sni)

		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", target.addr)
		if err != nil {
			continue
		}

		_ = conn.SetDeadline(time.Now().Add(timeout))

		// Send fragmented ClientHello
		offset := 0
		failed := false

		for offset < len(clientHello) {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				return candidate
			default:
			}

			chunkSize := randRange(minSize, maxSize)
			end := offset + chunkSize
			if end > len(clientHello) {
				end = len(clientHello)
			}

			_, writeErr := conn.Write(clientHello[offset:end])
			if writeErr != nil {
				failed = true
				break
			}
			offset = end

			if offset < len(clientHello) {
				sleepMs := randRange(minSleep, maxSleep)
				select {
				case <-ctx.Done():
					_ = conn.Close()
					return candidate
				case <-time.After(time.Duration(sleepMs) * time.Millisecond):
				}
			}
		}

		if !failed {
			// Read Server response (ServerHello or Alert)
			reply := make([]byte, 5)
			n, readErr := conn.Read(reply)
			_ = conn.Close()

			if (readErr == nil || n > 0) && n >= 1 && (reply[0] == 0x16 || reply[0] == 0x15 || reply[0] == 0x14 || reply[0] == 0x17) {
				dur := int(time.Since(start).Milliseconds())
				if dur <= 0 {
					dur = 1
				}
				totalLatency += dur
				successCount++
			}
		} else {
			_ = conn.Close()
		}
	}

	rate := (successCount * 100) / trials
	avgLat := 0
	if successCount > 0 {
		avgLat = totalLatency / successCount
	}

	candidate.SuccessRate = rate
	candidate.AvgLatency = avgLat

	// Calculate score: Higher success rate and lower latency
	if rate > 0 {
		latScore := 100 - (avgLat / 10)
		if latScore < 10 {
			latScore = 10
		}
		candidate.Score = (rate * 70 / 100) + (latScore * 30 / 100)
		if rate >= 75 {
			candidate.Status = "optimal"
		} else {
			candidate.Status = "passed"
		}
	} else {
		candidate.Score = 0
		candidate.Status = "failed"
	}

	return candidate
}

// ScanFragmentSettings benchmarks all fragment presets and identifies the optimal one
func ScanFragmentSettings(
	ctx context.Context,
	customCandidates []struct{ size, sleep string },
	onProgress func(FragmentScanProgress),
) ([]FragmentCandidate, *FragmentCandidate, error) {
	candidatesList := defaultFragmentCandidates
	if len(customCandidates) > 0 {
		candidatesList = customCandidates
	}

	var items []FragmentCandidate
	for idx, c := range candidatesList {
		items = append(items, FragmentCandidate{
			ID:         fmt.Sprintf("frag-%d", idx+1),
			SizeRange:  c.size,
			SleepRange: c.sleep,
		})
	}

	total := len(items)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []FragmentCandidate
	scannedCount := 0

	taskChan := make(chan FragmentCandidate, total)
	for _, it := range items {
		taskChan <- it
	}
	close(taskChan)

	concurrency := 4 // Low concurrency to avoid tripping rate limits

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range taskChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				tested := TestSingleFragmentCandidate(ctx, it, 2500*time.Millisecond)

				mu.Lock()
				scannedCount++
				results = append(results, tested)
				currentScanned := scannedCount

				// Copy and sort
				sorted := make([]FragmentCandidate, len(results))
				copy(sorted, results)
				sort.Slice(sorted, func(i, j int) bool {
					if sorted[i].SuccessRate != sorted[j].SuccessRate {
						return sorted[i].SuccessRate > sorted[j].SuccessRate
					}
					return sorted[i].AvgLatency < sorted[j].AvgLatency
				})

				var currentBest *FragmentCandidate
				if len(sorted) > 0 && sorted[0].SuccessRate > 0 {
					b := sorted[0]
					b.Recommended = true
					currentBest = &b
				}
				mu.Unlock()

				if onProgress != nil {
					onProgress(FragmentScanProgress{
						Total:       total,
						Scanned:     currentScanned,
						Current:     fmt.Sprintf("Size: %s, Sleep: %s", it.SizeRange, it.SleepRange),
						BestSetting: currentBest,
						Results:     sorted,
					})
				}
			}
		}()
	}

	wg.Wait()

	// Final Sort
	sort.Slice(results, func(i, j int) bool {
		if results[i].SuccessRate != results[j].SuccessRate {
			return results[i].SuccessRate > results[j].SuccessRate
		}
		return results[i].AvgLatency < results[j].AvgLatency
	})

	var best *FragmentCandidate
	if len(results) > 0 && results[0].SuccessRate > 0 {
		results[0].Recommended = true
		b := results[0]
		best = &b
	}

	return results, best, nil
}
