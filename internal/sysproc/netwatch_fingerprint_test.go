package sysproc

import (
	"net"
	"strings"
	"testing"
)

// The network watcher exists to notice a Wi-Fi switch or an unplugged cable. It
// must not notice NexVPN's own adapter, because that would make every connect
// and every disconnect trigger a tunnel re-verification in response to itself —
// the self-inflicted "Network change detected" line that follows each core
// start/stop in real logs.

func TestFingerprintInterfacesIgnoresNexVPNAdapters(t *testing.T) {
	real := []net.Interface{
		{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp},
		{Name: "NexVPN-TUN", Index: 42, Flags: net.FlagUp},
	}
	withGhost := []net.Interface{
		{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp},
		{Name: "NexVPN-TUN", Index: 42, Flags: net.FlagUp},
		{Name: "NexVPN-TUN-3", Index: 51, Flags: net.FlagUp | net.FlagMulticast},
		{Name: "NexVPN-OVPN", Index: 55, Flags: net.FlagUp},
	}

	baseline := fingerprintInterfaces([]net.Interface{{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp}})

	if got := fingerprintInterfaces(real); got != baseline {
		t.Errorf("our own adapter changed the fingerprint:\n  got  %q\n  want %q", got, baseline)
	}
	if got := fingerprintInterfaces(withGhost); got != baseline {
		t.Errorf("ghost and OpenVPN adapters changed the fingerprint:\n  got  %q\n  want %q", got, baseline)
	}
}

func TestFingerprintInterfacesStillSeesRealChanges(t *testing.T) {
	a := fingerprintInterfaces([]net.Interface{{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp}})
	b := fingerprintInterfaces([]net.Interface{{Name: "Wi-Fi", Index: 12, Flags: 0}})
	c := fingerprintInterfaces([]net.Interface{
		{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp},
		{Name: "Ethernet", Index: 3, Flags: net.FlagUp},
	})

	if a == b {
		t.Errorf("an interface going down must change the fingerprint — that transition is what breaks a TUN route")
	}
	if a == c {
		t.Errorf("a new third-party adapter must change the fingerprint")
	}
	if !strings.Contains(a, "Wi-Fi") {
		t.Errorf("fingerprint does not describe the real adapter: %q", a)
	}
}

func TestFingerprintInterfacesIsOrderIndependent(t *testing.T) {
	// Interface order is not stable across calls on Windows; without sorting, the
	// safety-net poller would report a change on every tick.
	x := []net.Interface{
		{Name: "Wi-Fi", Index: 12, Flags: net.FlagUp},
		{Name: "Bluetooth Network Connection", Index: 8, Flags: net.FlagUp},
	}
	y := []net.Interface{x[1], x[0]}

	if fingerprintInterfaces(x) != fingerprintInterfaces(y) {
		t.Errorf("the same adapter set produced two fingerprints")
	}
}
