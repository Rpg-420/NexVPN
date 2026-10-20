package protocols

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// TestWintunGhostDetectionLive probes the machine's real adapter state.
//
// It is env-gated because the answer is a property of the machine, not of the
// code: it is only meaningful on a Windows host that has run a TUN core at least
// once, and it needs to be told where wintun.dll lives because a `go test`
// binary cannot find the app's bin folder on its own.
//
// Run it when a connection reports `tun-ready` and then dies with a core error:
//
//	NEXVPN_WINTUN_BINDIR='D:\App1\build\bin\bin' \
//	  go test ./internal/protocols/ -run WintunGhostDetectionLive -v
//
// On the machine this was written for it reports `NexVPN-TUN: ghosted=true`,
// backed by a registry binding at
//
//	Control\Network\{4d36e972-...}\{A5EA6E0D-3CB3-1A48-8F33-E2252682EAB4}\Connection
//	    Name = NexVPN-TUN, PnPInstanceId = SWD\Wintun\{A5EA6E0D-...}
//
// while `Get-NetAdapter` cannot enumerate it and `openvpn --show-adapters` still
// lists it. The assertions below are the invariants of that rule rather than the
// machine-specific verdict, so the test is meaningful on any host.
func TestWintunGhostDetectionLive(t *testing.T) {
	binDir := strings.TrimSpace(os.Getenv("NEXVPN_WINTUN_BINDIR"))
	if binDir == "" {
		t.Skip("set NEXVPN_WINTUN_BINDIR to the folder holding wintun.dll to probe this machine")
	}
	if !WintunDLLPresent(binDir) {
		t.Fatalf("no wintun.dll in %s", binDir)
	}

	scan := ScanWintunGhosts()

	visible := make([]string, 0, len(scan.Visible))
	for n := range scan.Visible {
		visible = append(visible, n)
	}
	sort.Strings(visible)
	t.Logf("Windows can enumerate %d adapters: %v", len(visible), visible)

	bound := make([]string, 0, len(scan.Bound))
	for n := range scan.Bound {
		bound = append(bound, n)
	}
	sort.Strings(bound)
	t.Logf("the registry binds %d names to a Wintun device: %v", len(bound), bound)

	// The invariants that make the rule safe, checked against real data.
	for _, name := range append(append([]string{}, visible...), bound...) {
		ghosted := scan.Ghosted(name)
		if !ghosted {
			continue
		}
		if scan.Visible[strings.ToLower(name)] {
			t.Errorf("%q reported ghosted while Windows can enumerate it", name)
		}
		if !scan.Bound[strings.ToLower(name)] {
			t.Errorf("%q reported ghosted without a registry binding", name)
		}
	}

	for _, name := range []string{"NexVPN-TUN", "NexVPN-TUN-2", "NexVPN-TUN-3"} {
		t.Logf("%s: ghosted=%v", name, scan.Ghosted(name))
	}
}
