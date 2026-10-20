package core

import (
	"context"
	"net"
	"testing"

	"nexvpn/internal/config"
)

// localTestHost returns a non-loopback IPv4 address of this machine so these
// probes travel the same path as a real remote server.
//
// Loopback can no longer stand in for a live server: CanTCPProbe deliberately
// rejects 127.0.0.1, because the Aether profiles store it as their server and
// probing it would dial NexVPN's own proxy port instead of a server.
func localTestHost(t *testing.T) string {
	t.Helper()

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("cannot enumerate interfaces: %v", err)
	}
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() {
			continue
		}
		if v4 := ipn.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	t.Skip("no non-loopback IPv4 address on this machine")
	return ""
}

// startLiveTCPServer opens a listener on a real interface address and returns
// it. Connections are accepted and closed immediately, which is all a TCP probe
// needs to measure a real handshake.
func startLiveTCPServer(t *testing.T) (string, int) {
	t.Helper()

	host := localTestHost(t)
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Skipf("listen on %s: %v", host, err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	return host, ln.Addr().(*net.TCPAddr).Port
}

// deadEndpoint returns an address that was listening a moment ago and is now
// closed, so dialling it is refused immediately and deterministically. Using a
// blackhole address instead would make every test wait out the full probe
// timeout.
func deadEndpoint(t *testing.T) (string, int) {
	t.Helper()

	host := localTestHost(t)
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Skipf("listen on %s: %v", host, err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	return host, port
}

func ids(profiles []config.Profile) []string {
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.ID)
	}
	return out
}

func resultIDs(results []AutoSelectResult) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Profile.ID)
	}
	return out
}

func TestRankFailoverCandidates(t *testing.T) {
	profiles := []config.Profile{
		{ID: "failed", Name: "failed", Server: "10.0.0.1", LatencyMs: 10},
		{ID: "slow", Name: "slow", Server: "10.0.0.2", LatencyMs: 300},
		{ID: "fast", Name: "fast", Server: "10.0.0.3", LatencyMs: 40},
		{ID: "unmeasured", Name: "unmeasured", Server: "10.0.0.4"},
		{ID: "blank", Name: "blank"},
	}

	got := RankFailoverCandidates(profiles, "failed", 0)
	want := []string{"fast", "slow", "unmeasured"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", ids(got), want)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("candidate %d = %q, want %q (full order: %v)", i, got[i].ID, id, ids(got))
		}
	}
}

func TestRankFailoverCandidatesHonoursLimit(t *testing.T) {
	profiles := []config.Profile{
		{ID: "a", Server: "10.0.0.1", LatencyMs: 30},
		{ID: "b", Server: "10.0.0.2", LatencyMs: 10},
		{ID: "c", Server: "10.0.0.3", LatencyMs: 20},
	}

	got := RankFailoverCandidates(profiles, "", 2)
	if len(got) != 2 {
		t.Fatalf("limit ignored: got %v", ids(got))
	}
	if got[0].ID != "b" || got[1].ID != "c" {
		t.Errorf("limit kept the wrong pair: %v", ids(got))
	}
}

// A profile whose endpoint is buried in a pasted config (an imported .ovpn)
// cannot be probed, but it must still survive ranking as a last resort rather
// than being dropped — otherwise a user whose only profiles are imports would
// have no failover target at all.
func TestRankFailoverCandidatesPutsUnprobeableLast(t *testing.T) {
	profiles := []config.Profile{
		{ID: "ovpn", ConfigContent: "client\ndev tun\nremote 1.2.3.4 1194\n"},
		{ID: "measured", Server: "10.0.0.1", LatencyMs: 50},
	}

	got := RankFailoverCandidates(profiles, "", 0)
	if len(got) != 2 {
		t.Fatalf("got %v, want both profiles", ids(got))
	}
	if got[0].ID != "measured" {
		t.Errorf("a dialable profile must rank above an unprobeable one; got %v", ids(got))
	}
	if got[1].ID != "ovpn" {
		t.Errorf("the unprobeable profile should still be present; got %v", ids(got))
	}
}

func TestUnprobeableCandidates(t *testing.T) {
	candidates := []config.Profile{
		{ID: "dialable", Server: "10.0.0.1"},
		{ID: "ovpn", ConfigContent: "client\n"},
		{ID: "wg", Protocol: config.ProtocolWireGuard, Server: "10.0.0.9"},
	}

	got := UnprobeableCandidates(candidates)
	if len(got) != 2 || got[0].ID != "ovpn" || got[1].ID != "wg" {
		t.Fatalf("got %v, want [ovpn wg]", ids(got))
	}
}

func TestProbeReachableProfilesKeepsOnlyLiveServers(t *testing.T) {
	liveHost, livePort := startLiveTCPServer(t)
	deadHost, deadPort := deadEndpoint(t)

	profiles := []config.Profile{
		{ID: "dead", Name: "dead", Server: deadHost, Port: deadPort},
		{ID: "live", Name: "live", Server: liveHost, Port: livePort},
	}

	got := ProbeReachableProfiles(context.Background(), profiles)
	if len(got) != 1 {
		t.Fatalf("want exactly the live server, got %v", resultIDs(got))
	}
	if got[0].Profile.ID != "live" {
		t.Errorf("got %q, want \"live\"", got[0].Profile.ID)
	}
	if got[0].LatencyMs <= 0 {
		t.Errorf("latency must be positive for a reachable server, got %d", got[0].LatencyMs)
	}
}

// The regression this function exists for. The previous failover path took its
// choice from the cached LatencyMs field and never checked liveness, so it
// switched to a server that was already dead and burned the attempt budget. An
// empty probe result must stay empty — never quietly substituted with a guess.
func TestProbeReachableProfilesReturnsEmptyWhenNothingAnswers(t *testing.T) {
	host, deadPort := deadEndpoint(t)

	profiles := []config.Profile{
		{ID: "a", Name: "a", Server: host, Port: deadPort},
		{ID: "b", Name: "b", Server: host, Port: deadPort},
	}

	got := ProbeReachableProfiles(context.Background(), profiles)
	if len(got) != 0 {
		t.Fatalf("nothing answered, so the result must be empty — got %v", resultIDs(got))
	}
}

// FindFastestProfile deliberately keeps its "try the first one anyway" fallback
// because it backs the Connect button, where attempting something beats
// refusing outright. This test pins the difference so a later cleanup does not
// collapse the two behaviours into one.
func TestFindFastestProfileStillFallsBackWhenNothingAnswers(t *testing.T) {
	host, deadPort := deadEndpoint(t)

	profiles := []config.Profile{
		{ID: "a", Name: "a", Server: host, Port: deadPort},
	}

	best, latency, err := FindFastestProfile(context.Background(), profiles, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best == nil || best.ID != "a" {
		t.Fatalf("expected the first profile as a fallback, got %v", best)
	}
	if latency != 999 {
		t.Errorf("fallback latency = %d, want 999", latency)
	}
}

// CanTCPProbe decides whether a TCP dial can say anything at all about a
// profile. Getting this wrong is how auto-failover ended up either excluding
// live servers or treating our own proxy port as a healthy backup.
func TestCanTCPProbe(t *testing.T) {
	cases := []struct {
		name string
		prof config.Profile
		want bool
	}{
		{
			name: "ordinary remote endpoint",
			prof: config.Profile{Protocol: config.ProtocolSingBox, Server: "185.147.26.207", Port: 443},
			want: true,
		},
		{
			name: "endpoint reachable only through RawURI",
			prof: config.Profile{Protocol: config.ProtocolSingBox, RawURI: "vless://u@1.2.3.4:443#n"},
			want: true,
		},
		{
			// The Aether profiles store 127.0.0.1; probing it dials our own
			// proxy port, so a running core looked like a healthy server.
			name: "loopback endpoint",
			prof: config.Profile{Protocol: config.ProtocolAether, Server: "127.0.0.1", Port: 10808},
			want: false,
		},
		{
			name: "localhost by name",
			prof: config.Profile{Protocol: config.ProtocolSingBox, Server: "localhost", Port: 10808},
			want: false,
		},
		{
			// WireGuard/WARP endpoints listen on UDP only, so a refused TCP
			// dial is the expected result for a perfectly healthy server.
			name: "wireguard endpoint",
			prof: config.Profile{Protocol: config.ProtocolWireGuard, Server: "8.6.112.32", Port: 7306},
			want: false,
		},
		{
			name: "warp endpoint",
			prof: config.Profile{Protocol: config.ProtocolWARP, Server: "162.159.192.64", Port: 5279},
			want: false,
		},
		{
			name: "hysteria2 endpoint",
			prof: config.Profile{Protocol: config.ProtocolHysteria2, Server: "1.2.3.4", Port: 443},
			want: false,
		},
		{
			name: "psiphon placeholder host",
			prof: config.Profile{Protocol: config.ProtocolPsiphon, Server: "Psiphon Cloud Network"},
			want: false,
		},
		{
			name: "imported config with no address",
			prof: config.Profile{Protocol: config.ProtocolOpenVPN, ConfigContent: "client\ndev tun\n"},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanTCPProbe(tc.prof); got != tc.want {
				t.Errorf("CanTCPProbe = %v, want %v", got, tc.want)
			}
		})
	}
}

// A UDP-only endpoint must not be reported as dead: it lands in the
// last-resort bucket so triggerAutoFailover can still attempt it blindly.
func TestRankFailoverCandidatesTreatsUDPOnlyAsUnknown(t *testing.T) {
	profiles := []config.Profile{
		{ID: "wg", Protocol: config.ProtocolWARP, Server: "8.6.112.32", Port: 7306},
		{ID: "tcp", Protocol: config.ProtocolSingBox, Server: "10.0.0.1", LatencyMs: 40},
	}

	got := RankFailoverCandidates(profiles, "", 0)
	if len(got) != 2 {
		t.Fatalf("a UDP-only profile must stay a candidate; got %v", ids(got))
	}
	if got[0].ID != "tcp" {
		t.Errorf("the probeable profile should rank first; got %v", ids(got))
	}

	unknown := UnprobeableCandidates(got)
	if len(unknown) != 1 || unknown[0].ID != "wg" {
		t.Fatalf("the WARP profile should be the blind-attempt fallback; got %v", ids(unknown))
	}
}
