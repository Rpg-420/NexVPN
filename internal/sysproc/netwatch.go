package sysproc

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"nexvpn/internal/config"
)

// networkPollInterval is the safety-net period for detecting an interface
// change that the OS notification API did not report.
//
// The notification path is the fast one (it fires within milliseconds of a
// Wi-Fi switch); this ticker exists only so that a platform without
// notifications — or a notification registration that silently failed — still
// converges. 15 s is well below the engine's own health-check cadence, so it
// never becomes the thing that delays a reconnect, and a fingerprint comparison
// costs one netlink/GetAdaptersAddresses round trip.
const networkPollInterval = 15 * time.Second

// NetworkEvent reports that the machine's set of IP interfaces changed in a way
// that can invalidate an established tunnel: a Wi-Fi switch, a cable being
// unplugged, a VPN adapter appearing or disappearing, a DHCP lease moving.
//
// It deliberately does not try to describe *what* changed. The only safe
// consumer reaction is "re-verify the tunnel now", and any attempt to classify
// the change would be guesswork that the caller then has to trust.
type NetworkEvent struct {
	// Fingerprint is the adapter-set signature that produced this event.
	// Consecutive events always carry different fingerprints, so a consumer can
	// dedupe across the notify and poll paths without keeping its own copy.
	Fingerprint string

	// Source is "notify" for an OS notification and "poll" for the safety net.
	// It is logged rather than acted on; it exists so the diary can show
	// whether the fast path is actually working.
	Source string
}

// WatchNetworkChanges reports interface changes until ctx is cancelled, at
// which point the returned channel is closed.
//
// Two sources feed the channel: an OS notification where one exists (see
// registerInterfaceNotifications), plus a low-frequency fingerprint poller. The
// poller is not redundancy for its own sake — the notification APIs are
// documented to coalesce and can miss a rapid down/up pair, which is exactly
// the Wi-Fi-roaming case this feature exists for.
func WatchNetworkChanges(ctx context.Context) <-chan NetworkEvent {
	out := make(chan NetworkEvent, 4)

	// The OS callback runs on a thread Windows owns, so it must do the absolute
	// minimum: a non-blocking hand-off. Computing the fingerprint there would
	// run a syscall on a thread the network stack may be waiting on.
	wake := make(chan string, 8)
	registerInterfaceNotifications(ctx, func() {
		select {
		case wake <- "notify":
		default: // a wake-up is already pending; coalescing is the point
		}
	})

	go func() {
		defer close(out)

		emit := func(source, fp string) {
			select {
			case out <- NetworkEvent{Fingerprint: fp, Source: source}:
			default: // consumer is behind; one pending change is enough
			}
		}

		last := interfaceFingerprint()

		ticker := time.NewTicker(networkPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case source := <-wake:
				// Re-read rather than trusting the notification: several
				// notifications can arrive for one logical change, and the
				// fingerprint comparison collapses them.
				fp := interfaceFingerprint()
				if fp == "" || fp == last {
					continue
				}
				last = fp
				emit(source, fp)
			case <-ticker.C:
				fp := interfaceFingerprint()
				if fp == "" || fp == last {
					continue
				}
				last = fp
				emit("poll", fp)
			}
		}
	}()

	return out
}

// interfaceFingerprint builds a stable signature of the current adapter set.
//
// It uses only the standard library so that it behaves identically on every
// platform and cannot fail in a platform-specific way. Interface *order* is not
// stable across calls on Windows, so both the interface list and each
// interface's address list are sorted before joining; without that the poller
// would report a change on every tick.
//
// An empty return means the adapter set could not be read at all, which callers
// treat as "no information" rather than as a change.
func interfaceFingerprint() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	return fingerprintInterfaces(ifaces)
}

// fingerprintInterfaces builds the signature from an explicit interface list, so
// the ownership rule below can be tested without a real adapter set.
func fingerprintInterfaces(ifaces []net.Interface) string {
	parts := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		// NexVPN's own adapters are not part of "the network changed".
		//
		// Creating a TUN adapter is an interface change by any OS definition, so
		// without this exclusion every connect and every disconnect produced an
		// event, and the app reacted to its own work by re-verifying the tunnel —
		// visible in real logs as "Network change detected" within a second or two
		// of every core start/stop. The remaining case this cannot catch is a TAP
		// adapter under a generic name (an OpenVPN fallback lands on "Ethernet 2");
		// the watchdog's connection-state check covers that one instead.
		if config.IsNexVPNAdapterName(iface.Name) {
			continue
		}

		// The index and the up/down flag matter as much as the addresses: a
		// Wi-Fi radio going down keeps its address for a moment, and that
		// transition is precisely what breaks a TUN route.
		key := fmt.Sprintf("%s|%d|%d", iface.Name, iface.Index, iface.Flags)

		addrs, err := iface.Addrs()
		if err != nil {
			addrs = nil
		}
		addrParts := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addrParts = append(addrParts, addr.String())
		}
		sort.Strings(addrParts)

		parts = append(parts, key+"|"+strings.Join(addrParts, ","))
	}

	sort.Strings(parts)
	return strings.Join(parts, ";")
}
