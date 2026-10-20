package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
)

type QuotaAlertInfo struct {
	SubscriptionURL  string     `json:"subscription_url"`
	SubscriptionName string     `json:"subscription_name"`
	TotalBytes       int64      `json:"total_bytes"`
	UsedBytes        int64      `json:"used_bytes"`
	RemainingBytes   int64      `json:"remaining_bytes"`
	UsedPercent      float64    `json:"used_percent"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	DaysRemaining    int        `json:"days_remaining"`
	AlertTriggered   bool       `json:"alert_triggered"`
	AlertReason      string     `json:"alert_reason"`
}

type QuotaMonitor struct {
	store      *config.Store
	httpClient *http.Client
	mu         sync.Mutex
	lastAlerts map[string]time.Time
}

var (
	globalQuotaMonitor *QuotaMonitor
	quotaMonitorOnce   sync.Once
)

func GetQuotaMonitor(store *config.Store) *QuotaMonitor {
	quotaMonitorOnce.Do(func() {
		globalQuotaMonitor = &QuotaMonitor{
			store:      store,
			httpClient: newQuotaHTTPClient(),
			lastAlerts: make(map[string]time.Time),
		}
	})
	return globalQuotaMonitor
}

// newQuotaHTTPClient builds the client used for subscription quota lookups.
//
// http.Client.Timeout is deliberately NOT used here. It bounds connect, TLS,
// headers and the whole body as one number, so a slow-but-healthy endpoint is
// indistinguishable from a dead one — which is how the
// `[quota] Quota check failed ... context deadline exceeded` warning in the log
// was produced. Per-phase deadlines on the transport say which stage actually
// timed out, and the caller's context owns the overall budget.
//
// ProxyFromEnvironment is set explicitly: replacing http.DefaultTransport
// silently drops proxy support, and this app is routinely used behind one.
func newQuotaHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   8 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 12 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		},
	}
}

// CheckSubscriptionQuota queries the remote subscription endpoint and parses Subscription-Userinfo header
func (qm *QuotaMonitor) CheckSubscriptionQuota(subURL string) (*QuotaAlertInfo, error) {
	subURL = strings.TrimSpace(subURL)
	if subURL == "" {
		return nil, fmt.Errorf("empty subscription URL")
	}

	// The overall budget belongs to the caller's context, not to the client.
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", subURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "v2rayng/1.8.5")

	resp, err := qm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	userInfo := resp.Header.Get("Subscription-Userinfo")
	if userInfo == "" {
		userInfo = resp.Header.Get("subscription-userinfo")
	}

	info := &QuotaAlertInfo{
		SubscriptionURL:  subURL,
		SubscriptionName: "Subscription",
	}

	if userInfo == "" {
		return info, nil
	}

	// Parse header format: upload=1000; download=2000; total=10000000; expire=1700000000
	parts := strings.Split(userInfo, ";")
	var upload, download, total, expire int64
	for _, part := range parts {
		kv := strings.Split(strings.TrimSpace(part), "=")
		if len(kv) == 2 {
			k := strings.ToLower(kv[0])
			v, _ := strconv.ParseInt(kv[1], 10, 64)
			switch k {
			case "upload":
				upload = v
			case "download":
				download = v
			case "total":
				total = v
			case "expire":
				expire = v
			}
		}
	}

	info.TotalBytes = total
	info.UsedBytes = upload + download
	if total > 0 {
		info.RemainingBytes = total - info.UsedBytes
		if info.RemainingBytes < 0 {
			info.RemainingBytes = 0
		}
		info.UsedPercent = (float64(info.UsedBytes) / float64(total)) * 100.0
	}

	if expire > 0 {
		expTime := time.Unix(expire, 0)
		info.ExpiresAt = &expTime
		days := int(time.Until(expTime).Hours() / 24)
		if days < 0 {
			days = 0
		}
		info.DaysRemaining = days
	}

	// Evaluate alert threshold (>90% used or <3 days remaining)
	if (info.TotalBytes > 0 && info.UsedPercent >= 90.0) || (expire > 0 && info.DaysRemaining <= 3) {
		info.AlertTriggered = true
		if info.UsedPercent >= 90.0 {
			info.AlertReason = fmt.Sprintf("Subscription quota critical: %.1f%% used", info.UsedPercent)
		} else {
			info.AlertReason = fmt.Sprintf("Subscription expiring soon: %d days remaining", info.DaysRemaining)
		}
	}

	return info, nil
}

// DispatchWebhookAlert sends an alert payload to a custom HTTP/Telegram/Discord webhook
func (qm *QuotaMonitor) DispatchWebhookAlert(webhookURL string, alert QuotaAlertInfo) error {
	if strings.TrimSpace(webhookURL) == "" {
		return nil
	}

	payload := map[string]interface{}{
		"app":            "NexVPN",
		"event":          "quota_alert",
		"timestamp":      time.Now().Format(time.RFC3339),
		"reason":         alert.AlertReason,
		"used_percent":   alert.UsedPercent,
		"days_remaining": alert.DaysRemaining,
		"total_gb":       float64(alert.TotalBytes) / (1024 * 1024 * 1024),
		"remaining_gb":   float64(alert.RemainingBytes) / (1024 * 1024 * 1024),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", webhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := qm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// alertWindow throttles repeat alerts for the same subscription so the webhook
// is not hammered every cycle while a quota stays over the threshold.
const alertWindow = 24 * time.Hour

// shouldAlert reports whether an alert for subURL may be dispatched now, and
// records the dispatch time. This is what makes lastAlerts meaningful.
func (qm *QuotaMonitor) shouldAlert(subURL string) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if last, ok := qm.lastAlerts[subURL]; ok && time.Since(last) < alertWindow {
		return false
	}
	qm.lastAlerts[subURL] = time.Now()
	return true
}

// QuotaMonitorDaemon periodically checks every stored subscription against the
// configured quota threshold and fires the user's webhook once per alert window.
type QuotaMonitorDaemon struct {
	qm       *QuotaMonitor
	store    *config.Store
	cancel   context.CancelFunc
	mu       sync.RWMutex // RWMutex: IsRunning() reads state with RLock()
	running  bool
	interval time.Duration
	resetCh  chan time.Duration
	logFn    func(level, msg, tag string)
}

// NewQuotaMonitorDaemon creates the background quota watcher. Default cadence is
// every 6 hours; it can be changed at runtime via SetInterval.
func NewQuotaMonitorDaemon(store *config.Store, logFn func(level, msg, tag string)) *QuotaMonitorDaemon {
	return &QuotaMonitorDaemon{
		qm:       GetQuotaMonitor(store),
		store:    store,
		interval: 6 * time.Hour,
		resetCh:  make(chan time.Duration, 1),
		logFn:    logFn,
	}
}

func (d *QuotaMonitorDaemon) log(level, msg string) {
	if d.logFn != nil {
		d.logFn(level, msg, "quota")
	}
}

// SetInterval changes the check cadence. A running daemon picks the new value up
// immediately instead of waiting for a restart.
func (d *QuotaMonitorDaemon) SetInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	d.mu.Lock()
	d.interval = interval
	running := d.running
	d.mu.Unlock()
	if running {
		select {
		case d.resetCh <- interval:
		default:
		}
	}
}

func (d *QuotaMonitorDaemon) Start() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.running = true
	interval := d.interval
	d.mu.Unlock()

	go d.runLoop(ctx, interval)
}

func (d *QuotaMonitorDaemon) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	cancel := d.cancel
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (d *QuotaMonitorDaemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

func (d *QuotaMonitorDaemon) runLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run one pass shortly after start so a user who just opened the app gets
	// an immediate answer instead of waiting a full cycle.
	initial := time.NewTimer(20 * time.Second)
	defer initial.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
			d.CheckAll()
		case <-ticker.C:
			d.CheckAll()
		case newInterval := <-d.resetCh:
			if newInterval > 0 {
				ticker.Reset(newInterval)
			}
		}
	}
}

// CheckAll evaluates every stored subscription and dispatches the webhook for
// any that crossed the configured threshold.
func (d *QuotaMonitorDaemon) CheckAll() {
	if d.store == nil {
		return
	}
	urls := d.store.GetSubscriptionURLs()
	if len(urls) == 0 {
		return
	}

	settings := d.store.GetSettings()
	threshold := float64(settings.QuotaAlertPercentage)
	if threshold <= 0 || threshold > 100 {
		threshold = 90.0
	}

	for _, url := range urls {
		info, err := d.qm.CheckSubscriptionQuota(url)
		if err != nil {
			d.log("warn", fmt.Sprintf("Quota check failed for %s: %v", RedactURLSecrets(url), err))
			continue
		}
		if info == nil || !info.AlertTriggered {
			continue
		}
		// Re-evaluate against the user's configured percentage.
		if info.TotalBytes > 0 && info.UsedPercent < threshold && info.DaysRemaining > 3 {
			continue
		}
		if !d.qm.shouldAlert(url) {
			continue
		}

		d.log("warn", fmt.Sprintf("Subscription alert: %s (%s)", info.AlertReason, RedactURLSecrets(url)))
		if strings.TrimSpace(settings.QuotaWebhookURL) != "" {
			if err := d.qm.DispatchWebhookAlert(settings.QuotaWebhookURL, *info); err != nil {
				d.log("error", fmt.Sprintf("Failed to dispatch quota webhook: %v", err))
			} else {
				d.log("info", "Quota webhook alert dispatched")
			}
		}
	}
}
