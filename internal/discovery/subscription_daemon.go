package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/parser"
)

// SubscriptionDaemon coordinates periodic background updates for subscription URLs
type SubscriptionDaemon struct {
	store    *config.Store
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	running  bool
	interval time.Duration
	logFn    func(level, msg, tag string)
	// resetCh nudges runLoop to re-arm its ticker when the interval is changed
	// at runtime. Buffered (size 1) so a change never blocks the caller.
	resetCh chan struct{}
}

func NewSubscriptionDaemon(store *config.Store, logFn func(level, msg, tag string)) *SubscriptionDaemon {
	return &SubscriptionDaemon{
		store:    store,
		interval: 2 * time.Hour,
		logFn:    logFn,
		resetCh:  make(chan struct{}, 1),
	}
}

// SetInterval updates the background refresh interval. If the daemon is already
// running the change is applied immediately by re-arming the ticker, instead of
// only taking effect after the next restart.
func (d *SubscriptionDaemon) SetInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	d.mu.Lock()
	changed := d.interval != interval
	d.interval = interval
	running := d.running
	d.mu.Unlock()

	if changed && running && d.resetCh != nil {
		select {
		case d.resetCh <- struct{}{}:
		default:
		}
	}
}

// currentInterval returns the configured interval under lock, falling back to a
// sane default if it was never set.
func (d *SubscriptionDaemon) currentInterval() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.interval <= 0 {
		return 2 * time.Hour
	}
	return d.interval
}

func (d *SubscriptionDaemon) log(level, msg string) {
	if d.logFn != nil {
		d.logFn(level, msg, "sub-daemon")
	}
}

// Start launches the background updater
func (d *SubscriptionDaemon) Start() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.running = true
	d.mu.Unlock()

	go d.runLoop()
}

// Stop terminates the background updater
func (d *SubscriptionDaemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	d.running = false
}

func (d *SubscriptionDaemon) runLoop() {
	// Perform an initial check 30 seconds after app startup
	select {
	case <-time.After(30 * time.Second):
		d.RefreshAllSubscriptions()
	case <-d.ctx.Done():
		return
	}

	ticker := time.NewTicker(d.currentInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.RefreshAllSubscriptions()
		case <-d.resetCh:
			// Interval changed while running: rebuild the ticker so the new
			// value takes effect now rather than after the next app restart.
			ticker.Stop()
			ticker = time.NewTicker(d.currentInterval())
			d.log("info", fmt.Sprintf("Subscription auto-update interval changed to %s", d.currentInterval()))
		case <-d.ctx.Done():
			return
		}
	}
}

// RefreshAllSubscriptions finds all subscription profiles and fetches fresh node configs
func (d *SubscriptionDaemon) RefreshAllSubscriptions() {
	if d.store == nil {
		return
	}

	profiles := d.store.GetProfiles()
	seenURLs := make(map[string]bool)

	for _, p := range profiles {
		subURL := strings.TrimSpace(p.SubscriptionURL)
		if subURL == "" || seenURLs[subURL] || (!strings.HasPrefix(subURL, "http://") && !strings.HasPrefix(subURL, "https://")) {
			continue
		}
		seenURLs[subURL] = true

		go func(url string, groupName string) {
			// Subscription URLs are credentials: the token/key usually lives in the
			// query string. Always log a masked form — this must not depend on the
			// user's Privacy Shield setting.
			safeURL := core.RedactURLSecrets(url)
			d.log("info", fmt.Sprintf("Background refreshing subscription: %s", safeURL))
			nodes, err := parser.FetchSubscription(url)
			if err != nil {
				d.log("warn", fmt.Sprintf("Failed background subscription refresh (%s): %v", safeURL, err))
				return
			}

			if len(nodes) > 0 {
				var newProfiles []config.Profile
				for _, node := range nodes {
					newProfiles = append(newProfiles, config.Profile{
						Name:             node.Name,
						Protocol:         node.Protocol,
						Category:         node.Category,
						ConfigContent:    node.ConfigContent,
						RawURI:           node.RawURI,
						Server:           node.Server,
						Port:             node.Port,
						Group:            groupName,
						SubscriptionURL:  url,
						SubscriptionName: groupName,
						CreatedAt:        time.Now(),
					})
				}
				if err := d.store.ReplaceSubscriptionProfiles(url, newProfiles); err != nil {
					d.log("warn", fmt.Sprintf("Failed to save updated subscription profiles (%s): %v", safeURL, err))
				} else {
					d.log("info", fmt.Sprintf("Successfully refreshed and updated %d profiles from subscription: %s", len(newProfiles), safeURL))
				}
			}
		}(subURL, p.Group)
	}
}
