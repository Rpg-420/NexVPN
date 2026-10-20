//go:build windows

package protocols

import "testing"

// TestWintunGhostScanGhosted pins the ghost rule against the evidence that
// produced it.
//
// The rule is deliberately a two-part conjunction: the registry must bind the
// name to a Wintun device AND Windows must not be able to enumerate an adapter
// under it. Each half alone gives the wrong answer, and both wrong answers were
// observed on a real machine:
//
//   - "the registry binds it" alone marks a healthy, live adapter as a ghost, so
//     the app would rename its adapter on every single launch.
//   - "the Wintun DLL answers for the name" alone — the original rule — reported
//     `ghosted=false` for a name that was in fact bound, so the avoidance never
//     fired. `WintunOpenAdapter("NexVPN-TUN")` returned NULL for that ghost.
//
// An unknown half must yield false. A probe that did not run must never be able
// to rename every adapter.
func TestWintunGhostScanGhosted(t *testing.T) {
	visible := map[string]bool{"wi-fi": true, "ethernet": true, "nexvpn-tun-2": true}
	bound := map[string]bool{"nexvpn-tun": true, "nexvpn-tun-2": true}

	cases := []struct {
		name  string
		scan  WintunGhostScan
		query string
		want  bool
	}{
		{
			name:  "bound but not enumerable is a ghost",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "NexVPN-TUN",
			want:  true,
		},
		{
			name:  "bound and enumerable is a live adapter, not a ghost",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "NexVPN-TUN-2",
			want:  false,
		},
		{
			name:  "name matching is case-insensitive",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "nExVpN-tUn",
			want:  true,
		},
		{
			name:  "surrounding whitespace is ignored",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "  NexVPN-TUN  ",
			want:  true,
		},
		{
			name:  "not bound at all is free, not ghosted",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "NexVPN-TUN-7",
			want:  false,
		},
		{
			name:  "empty name is never ghosted",
			scan:  WintunGhostScan{Visible: visible, Bound: bound},
			query: "   ",
			want:  false,
		},
		{
			name:  "unknown adapter list means assume nothing is ghosted",
			scan:  WintunGhostScan{Visible: nil, Bound: bound},
			query: "NexVPN-TUN",
			want:  false,
		},
		{
			name:  "unknown registry bindings means assume nothing is ghosted",
			scan:  WintunGhostScan{Visible: visible, Bound: nil},
			query: "NexVPN-TUN",
			want:  false,
		},
		{
			name:  "both halves unknown means assume nothing is ghosted",
			scan:  WintunGhostScan{},
			query: "NexVPN-TUN",
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scan.Ghosted(tc.query); got != tc.want {
				t.Errorf("Ghosted(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}
