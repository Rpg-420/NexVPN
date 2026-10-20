package core

import (
	"fmt"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
)

// RevalidateTunnel re-checks an established tunnel after an event that commonly
// breaks one — a Wi-Fi switch, a cable being unplugged, the machine waking from
// sleep — and tears it down only when it can prove the tunnel is gone.
//
// The asymmetry matters. A false positive here disconnects a user whose
// connection was about to recover on its own, which is far more annoying than
// the few seconds the periodic health monitor would have taken to notice. So
// the function waits for the network to settle, requires a positive failure
// signal, and does nothing at all when the state is not "connected".
func (e *Engine) RevalidateTunnel(reason string) {
	e.mu.RLock()
	state := e.status.State
	profile := e.activeProfile
	e.mu.RUnlock()

	if state != StateConnected {
		return
	}

	diary.Log("net", "revalidate", "reason", reason, "protocol", string(profile.Protocol))

	// A Wi-Fi switch reports the interface change before the new interface has
	// an address or a default route. Probing inside that window would fail every
	// time and turn a normal roam into a disconnect.
	time.Sleep(2500 * time.Millisecond)

	// Re-read under the lock: the wait above is long enough for the user to have
	// hit disconnect, and acting on a stale snapshot would resurrect a session
	// they just ended.
	e.mu.RLock()
	state = e.status.State
	adapter := e.activeAdapter
	profile = e.activeProfile
	e.mu.RUnlock()

	if state != StateConnected {
		return
	}

	if adapter == nil || !adapter.IsRunning() {
		e.AddLog("warning", fmt.Sprintf(
			"Core process stopped after %s — reconnecting", reason), "engine")
		diary.Log("net", "revalidate-core-dead", "reason", reason)
		e.setErrorStatus(fmt.Sprintf("Core stopped after %s", reason))
		return
	}

	// For every core except OpenVPN the loopback inbound is the cheapest
	// end-to-end proof that the core is still forwarding traffic rather than
	// merely alive. OpenVPN has no local proxy port, so its liveness check above
	// is all there is — which is why that check is not skipped for it.
	if profile.Protocol != config.ProtocolOpenVPN {
		if !probeProxy(10808, 6*time.Second) {
			e.AddLog("warning", fmt.Sprintf(
				"Local proxy stopped answering after %s — reconnecting", reason), "engine")
			diary.Log("net", "revalidate-proxy-dead", "reason", reason)
			e.setErrorStatus(fmt.Sprintf("Tunnel lost after %s", reason))
			return
		}
	}

	diary.Log("net", "revalidate-ok", "reason", reason)
	e.AddLog("info", fmt.Sprintf("Tunnel verified after %s", reason), "engine")
}

// SessionProtocol reports the protocol of the active session, or an empty
// string when nothing is connected. It exists so the network watchdog can make
// a decision without reaching into engine internals.
func (e *Engine) SessionProtocol() config.Protocol {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.status.State != StateConnected {
		return ""
	}
	return e.activeProfile.Protocol
}
