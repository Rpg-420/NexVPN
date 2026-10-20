package core

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type HealthStatus struct {
	QualityScore     int       `json:"quality_score"` // 0 - 100
	QualityLevel     string    `json:"quality_level"` // "excellent", "good", "fair", "poor", "unhealthy"
	LatencyMs        int       `json:"latency_ms"`
	JitterMs         int       `json:"jitter_ms"`
	PacketLossPct    float64   `json:"packet_loss_pct"`
	ConsecutiveFails int       `json:"consecutive_fails"`
	IsAlive          bool      `json:"is_alive"`
	LastChecked      time.Time `json:"last_checked"`
}

type HealthMonitor struct {
	mu               sync.RWMutex
	status           HealthStatus
	history          []int
	consecutiveFails int
	probeURL         string
	proxyPort        int
	onStatusChange   func(HealthStatus)
	onUnhealthy      func(consecutiveFails int)
	// unhealthyNotified guards onUnhealthy so it fires once per outage episode
	// rather than on every probe tick while the connection stays down.
	unhealthyNotified bool
}

func NewHealthMonitor(probeURL string, proxyPort int) *HealthMonitor {
	if probeURL == "" {
		probeURL = "http://cp.cloudflare.com/generate_204"
	}
	if proxyPort <= 0 {
		proxyPort = 10808
	}
	return &HealthMonitor{
		probeURL:  probeURL,
		proxyPort: proxyPort,
		history:   make([]int, 0, 10),
		status: HealthStatus{
			QualityScore: 100,
			QualityLevel: "excellent",
			IsAlive:      true,
			LastChecked:  time.Now(),
		},
	}
}

func (hm *HealthMonitor) SetCallback(cb func(HealthStatus)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onStatusChange = cb
}

func (hm *HealthMonitor) SetUnhealthyCallback(cb func(int)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onUnhealthy = cb
}

func (hm *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Initial probe after a short delay. The timer is tracked so it can be
	// cancelled when the connection ends — otherwise a stale probe fires
	// against a torn-down proxy port.
	initialTimer := time.AfterFunc(2*time.Second, func() {
		select {
		case <-ctx.Done():
			return
		default:
			hm.probe()
		}
	})
	defer initialTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.probe()
		}
	}
}

func (hm *HealthMonitor) probe() {
	proxyScheme := "socks5"
	port := hm.proxyPort
	if port <= 0 {
		port = 10808
	}
	if port == 10809 {
		proxyScheme = "http"
	}
	proxyUrl, _ := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", proxyScheme, port))
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		DialContext: (&net.Dialer{
			Timeout: 4 * time.Second,
		}).DialContext,
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
	}

	start := time.Now()
	resp, err := client.Get(hm.probeURL)
	latency := int(time.Since(start).Milliseconds())
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	if err != nil || (resp != nil && resp.StatusCode >= 400 && resp.StatusCode != 404) {
		hm.consecutiveFails++
		hm.status.IsAlive = false
		hm.status.ConsecutiveFails = hm.consecutiveFails
		hm.status.LastChecked = time.Now()

		if hm.consecutiveFails >= 3 {
			hm.status.QualityScore = 0
			hm.status.QualityLevel = "unhealthy"
			if hm.onUnhealthy != nil && !hm.unhealthyNotified {
				hm.unhealthyNotified = true
				go hm.onUnhealthy(hm.consecutiveFails)
			}
		} else {
			hm.status.QualityScore = int(math.Max(0, float64(hm.status.QualityScore-30)))
			hm.status.QualityLevel = "poor"
		}
	} else {
		hm.consecutiveFails = 0
		hm.unhealthyNotified = false
		hm.status.ConsecutiveFails = 0
		hm.status.IsAlive = true
		hm.status.LatencyMs = latency
		hm.status.LastChecked = time.Now()

		// Track jitter
		if len(hm.history) >= 10 {
			hm.history = hm.history[1:]
		}
		if len(hm.history) > 0 {
			lastLat := hm.history[len(hm.history)-1]
			hm.status.JitterMs = int(math.Abs(float64(latency - lastLat)))
		}
		hm.history = append(hm.history, latency)

		// Calculate Quality Score (0-100)
		// score starts at 100 (excellent); the ladder below only lowers it.
		score := 100
		if latency >= 700 {
			score = 25
		} else if latency >= 400 {
			score = 45
		} else if latency >= 250 {
			score = 80
		} else if latency >= 150 {
			score = 90
		}

		// Penalize jitter
		if hm.status.JitterMs > 100 {
			score -= 15
		} else if hm.status.JitterMs > 50 {
			score -= 8
		}

		if score < 0 {
			score = 0
		}

		hm.status.QualityScore = score
		if score >= 85 {
			hm.status.QualityLevel = "excellent"
		} else if score >= 70 {
			hm.status.QualityLevel = "good"
		} else if score >= 45 {
			hm.status.QualityLevel = "fair"
		} else {
			hm.status.QualityLevel = "poor"
		}
	}

	if hm.onStatusChange != nil {
		cbCopy := hm.status
		go hm.onStatusChange(cbCopy)
	}
}

func (hm *HealthMonitor) GetStatus() HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.status
}
