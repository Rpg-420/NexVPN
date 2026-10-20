package main

import (
	"context"
	"fmt"
	"os/exec"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
)

// netWatchDebounce collapses a burst of interface notifications into a single
// revalidation.
//
// One Wi-Fi switch produces several notifications (the old interface goes down,
// addresses are removed, the new interface comes up, addresses are added), and
// reacting to each one would run the revalidation half a dozen times, each with
// its own settle delay. Two and a half seconds is longer than the burst and far
// shorter than the health monitor's cadence, so nothing is missed.
const netWatchDebounce = 2500 * time.Millisecond

// scheduleTickInterval is how often the schedule rules are evaluated. A minute
// would be enough for the one-minute slot granularity, but a 30 s tick means a
// rule can fire up to 30 s late rather than up to 60 s late.
const scheduleTickInterval = 30 * time.Second

// runStartupRepair fixes anything a previous, uncleanly-terminated run left
// behind.
//
// This runs before the tray is created and before any daemon starts, because
// everything it repairs (a core holding 10808, a ghost TUN adapter, a system
// proxy pointing at a dead port, a half-applied kill switch) would otherwise
// break those very components in ways that look unrelated.
func (a *App) runStartupRepair() {
	journal, actions, found := core.AutoRepairFromJournal()
	if !found {
		return
	}

	a.engine.AddLog("warning", fmt.Sprintf(
		"Previous session ended unexpectedly (stage: %s, server: %s). Repaired automatically.",
		journal.Stage, journal.Server), "startup")
	for _, action := range actions {
		a.engine.AddLog("info", "Recovery: "+action, "startup")
	}
}

// currentSSID returns the SSID of the joined Wi-Fi network, or an empty string
// when the machine is on Ethernet, has no wireless adapter, or the value cannot
// be read.
//
// An empty return means "no information", and every caller treats it that way:
// a rule is never matched against it, because doing so would make an SSID rule
// fire on a wired connection.
func currentSSID() string {
	if goruntime.GOOS != "windows" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "netsh", "wlan", "show", "interfaces")
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// The output is one block per interface with lines like
	//     SSID                   : HomeNetwork
	//     BSSID                  : aa:bb:cc:dd:ee:ff
	// so the BSSID line must be excluded explicitly: it also starts with "SSID"
	// if the check is a naive HasPrefix on the trimmed line.
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "SSID") || strings.HasPrefix(line, "BSSID") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if ssid := strings.TrimSpace(parts[1]); ssid != "" {
			return ssid
		}
	}
	return ""
}

// CurrentNetworkSSID exposes the detected SSID to the interface so the settings
// editor can offer "use the network I am on right now".
func (a *App) CurrentNetworkSSID() string {
	return currentSSID()
}

// evaluateNetworkRules applies the user's SSID automation.
//
// When several rules match, the first enabled one wins. That ordering is the
// only one a user can reason about: the list is shown in order, and "the top
// matching rule applies" is a rule they can verify by looking at it.
func (a *App) evaluateNetworkRules(ssid string, reason string) {
	if ssid == "" {
		return
	}

	settings := a.store.GetSettings()
	if len(settings.NetworkRules) == 0 {
		return
	}

	var matched *config.NetworkRule
	for i := range settings.NetworkRules {
		if settings.NetworkRules[i].MatchesNetwork(ssid) {
			matched = &settings.NetworkRules[i]
			break
		}
	}
	if matched == nil {
		return
	}

	state := a.GetConnectionStatus().State
	diary.Log("net", "ssid-rule",
		"ssid", ssid,
		"action", matched.Action,
		"state", string(state),
		"reason", reason)

	switch matched.Action {
	case config.NetworkActionDirect:
		// A trusted network: drop the tunnel if one is up, and do not
		// auto-connect. Skipping this when already disconnected keeps the rule
		// idempotent, so it can be evaluated on every network event.
		if state == "connected" || state == "connecting" {
			a.engine.AddLog("info", fmt.Sprintf(
				"Network \"%s\" is set to direct — disconnecting", ssid), "net")
			_ = a.Disconnect()
		}
	case config.NetworkActionConnect:
		if state == "disconnected" || state == "error" {
			a.engine.AddLog("info", fmt.Sprintf(
				"Network \"%s\" matched a connect rule — connecting", ssid), "net")
			go func() {
				// Give the interface a moment to finish coming up. Connecting
				// into a half-configured adapter is the classic reason an
				// auto-connect rule appears not to work.
				time.Sleep(2 * time.Second)
				target := settings.LastProfileID
				if target == "" {
					if profiles := a.store.GetProfiles(); len(profiles) > 0 {
						target = profiles[0].ID
					}
				}
				if target != "" {
					_ = a.Connect(target)
				}
			}()
		}
	case config.NetworkActionSkip:
		// Explicitly do nothing: the rule exists to stop a broader rule from
		// applying, which is only meaningful because the first match wins.
	}
}

// startWatchdogs starts the OS event watchers and the schedule ticker.
//
// Every watcher here is an optimisation over something that already works: the
// health monitor notices a dead tunnel, and auto-connect runs at launch. What
// they buy is latency — reacting in about a second instead of at the next
// health check — so none of them is allowed to be load-bearing, and each fails
// silently on a platform that cannot provide it.
func (a *App) startWatchdogs(ctx context.Context) {
	a.startNetworkWatchdog(ctx)
	a.startPowerWatchdog(ctx)
	a.startScheduleLoop(ctx)
	a.startIdleCoreSweeper(ctx)
}

// idleCoreSweepInterval is how often the app looks for core processes that
// outlived their session.
//
// Two minutes is the compromise: short enough that a user who disconnects and
// reconnects a minute later is not fighting an orphan for the port, long enough
// that the check is invisible in every other respect. It is a process scan plus a
// map lookup, not a network operation.
const idleCoreSweepInterval = 2 * time.Minute

// startIdleCoreSweeper reaps core processes no session owns.
//
// The engine already sweeps at startup, at the beginning of every connect, on
// shutdown and during repair — all of which are moments the user is waiting in.
// This is the one that runs while nobody is doing anything, so the debris is gone
// before the next attempt has to explain it away.
func (a *App) startIdleCoreSweeper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(idleCoreSweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if swept := a.engine.SweepIdleCores(); swept > 0 {
					a.engine.AddLog("info", fmt.Sprintf(
						"Background maintenance: %d orphaned core process(es) reaped while disconnected", swept), "maint")
				}
			}
		}
	}()
}

func (a *App) startNetworkWatchdog(ctx context.Context) {
	events := sysproc.WatchNetworkChanges(ctx)

	go func() {
		// The SSID at launch is evaluated once directly, so a rule for the
		// network the machine is already on applies even though no change event
		// will ever fire for it.
		initial := currentSSID()
		if initial != "" {
			diary.Log("net", "ssid-initial", "ssid", initial)
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "network:ssid", initial)
			}
			a.evaluateNetworkRules(initial, "startup")
		}

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}

				// Debounce: collapse the burst of notifications one interface
				// change produces into a single reaction.
				a.netWatchMu.Lock()
				if time.Since(a.lastNetEvent) < netWatchDebounce {
					a.netWatchMu.Unlock()
					continue
				}
				a.lastNetEvent = time.Now()
				a.netWatchMu.Unlock()

				diary.Log("net", "change", "source", event.Source)

				// Only a live session can be broken by a network change.
				//
				// Announcing "verifying the tunnel" while nothing is connected —
				// which is what happened on every teardown, because the adapter and
				// route changes arrive then — made routine disconnects read like a
				// fault in the log, and the revalidation it promised was a no-op the
				// engine would immediately skip. The SSID rules are still evaluated
				// either way: joining a known network is exactly when a connect rule
				// is supposed to fire.
				if a.engine.SessionProtocol() == "" {
					diary.Log("net", "change-no-session", "source", event.Source)
				} else {
					a.engine.AddLog("info", "Network change detected — verifying the tunnel", "net")
					// The tunnel check is slow by design (it waits for the network to
					// settle), so it must not block the next event.
					go a.engine.RevalidateTunnel("network change")
				}

				ssid := currentSSID()
				if a.appCtx() != nil {
					runtime.EventsEmit(a.appCtx(), "network:ssid", ssid)
				}
				a.evaluateNetworkRules(ssid, "network change")
			}
		}
	}()
}

func (a *App) startPowerWatchdog(ctx context.Context) {
	events := sysproc.WatchPowerEvents(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}

				diary.Log("power", event.Kind)
				if a.appCtx() != nil {
					runtime.EventsEmit(a.appCtx(), "power:event", event.Kind)
				}

				switch event.Kind {
				case sysproc.PowerResume:
					// Sockets do not survive suspend, and a core can die while
					// the system is asleep without its exit callback ever being
					// delivered. Resuming is therefore the moment most likely to
					// find a tunnel that looks connected and is not.
					a.engine.AddLog("info", "System resumed — verifying the tunnel", "power")
					go a.engine.RevalidateTunnel("system resume")
				case sysproc.PowerSuspend:
					a.engine.AddLog("info", "System is suspending", "power")
				case sysproc.PowerAC, sysproc.PowerBattery:
					// Informational only. The network watcher covers the case
					// where a power change is accompanied by a real interface
					// change; reacting here as well would double up on it.
					a.engine.AddLog("info", fmt.Sprintf("Power source changed to %s", event.Kind), "power")
				}
			}
		}
	}()
}

// startScheduleLoop fires the user's scheduled connect/disconnect rules.
func (a *App) startScheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(scheduleTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runDueSchedules(time.Now())
		}
	}
}

// runDueSchedules is separated from the ticker so it can be tested and so the
// scheduling decision is visible in one place.
func (a *App) runDueSchedules(now time.Time) {
	settings := a.store.GetSettings()
	if len(settings.ScheduleRules) == 0 {
		return
	}

	changed := false
	for i := range settings.ScheduleRules {
		rule := &settings.ScheduleRules[i]
		due, slot := rule.ShouldFireAt(now)
		if !due {
			continue
		}

		// Record the slot before acting. If the action fails or the process
		// dies mid-action, the rule must not re-fire every 30 seconds for the
		// rest of the minute — a connect/disconnect loop is worse than a
		// missed schedule.
		rule.LastRun = slot
		changed = true

		diary.Log("schedule", "fire",
			"action", rule.Action,
			"time", rule.Time,
			"profile", rule.ProfileID)

		switch rule.Action {
		case config.ScheduleActionConnect:
			state := a.GetConnectionStatus().State
			if state == "connected" || state == "connecting" {
				continue
			}
			target := rule.ProfileID
			if target == "" {
				target = settings.LastProfileID
			}
			if target == "" {
				if profiles := a.store.GetProfiles(); len(profiles) > 0 {
					target = profiles[0].ID
				}
			}
			if target != "" {
				a.engine.AddLog("info", fmt.Sprintf(
					"Scheduled connect (%s)", rule.Time), "schedule")
				go func(id string) { _ = a.Connect(id) }(target)
			}
		case config.ScheduleActionDisconnect:
			state := a.GetConnectionStatus().State
			if state != "connected" && state != "connecting" {
				continue
			}
			a.engine.AddLog("info", fmt.Sprintf(
				"Scheduled disconnect (%s)", rule.Time), "schedule")
			go func() { _ = a.Disconnect() }()
		}
	}

	if changed {
		if err := a.store.SaveSettings(settings); err != nil {
			a.engine.AddLog("error", fmt.Sprintf(
				"Could not persist the schedule run marker: %v", err), "schedule")
		}
	}
}
