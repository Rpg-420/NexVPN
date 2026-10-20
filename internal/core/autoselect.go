package core

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

// AutoSelectResult pairs a profile with the latency measured for it.
type AutoSelectResult struct {
	Profile   *config.Profile `json:"profile"`
	LatencyMs int             `json:"latency_ms"`
}

// probeTimeout bounds a single TCP probe. Long enough to cross a slow mobile
// link, short enough that a handful of dead candidates does not stall a
// failover attempt.
const probeTimeout = 1800 * time.Millisecond

// resolveProbeTarget returns the address a reachability probe would dial for a
// profile, or an empty server when the profile carries none we can use.
//
// Profiles created by ImportProfile and the OpenVPN importer hold only
// ConfigContent — the endpoint lives inside the pasted config body and is never
// extracted — so they resolve to "". Callers must treat that as "unknown", not
// as "dead"; see UnprobeableCandidates.
func resolveProbeTarget(prof config.Profile) (string, int) {
	server := prof.Server
	port := prof.Port

	if server == "" && prof.RawURI != "" {
		if node, err := parser.ParseURI(prof.RawURI); err == nil && node != nil {
			server = node.Server
			port = node.Port
		}
	}
	if port <= 0 {
		port = 443
	}
	return server, port
}

// IsProbeable reports whether a profile carries an address a reachability probe
// can dial.
//
// Prefer CanTCPProbe for anything that decides whether a server is *alive*: an
// address that exists is not the same as an endpoint a TCP probe can conclude
// anything from.
func IsProbeable(prof config.Profile) bool {
	server, _ := resolveProbeTarget(prof)
	return server != ""
}

// udpOnlyProtocols never accept TCP on their endpoint port, so a refused TCP
// dial says nothing about whether the server is up. WARP/WireGuard endpoints in
// particular listen only on UDP.
var udpOnlyProtocols = map[config.Protocol]bool{
	config.ProtocolWireGuard: true,
	config.ProtocolWARP:      true,
	config.ProtocolHysteria2: true,
	config.ProtocolTUIC:      true,
	config.ProtocolAether:    true,
	config.ProtocolPsiphon:   true,
}

// isLoopbackHost reports whether a profile points back at this machine.
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}

// CanTCPProbe reports whether a plain TCP dial to this profile's endpoint can
// tell us anything at all.
//
// Two kinds of profile can never pass a TCP probe, and in neither case is that
// evidence the server is dead:
//
//   - UDP-only protocols (WireGuard/WARP, Hysteria2, TUIC, and the Aether and
//     Psiphon engines). A refused dial is the expected result for a healthy one.
//   - loopback endpoints. The Aether profiles store "127.0.0.1" as their server,
//     so probing them dialled NexVPN's own proxy port: a core that happened to be
//     running answered instantly and made them look like the healthiest backup
//     server on the list.
//
// Both are treated as *unknown*, never as *dead*, which keeps them eligible for
// the last-resort blind attempt in triggerAutoFailover.
func CanTCPProbe(prof config.Profile) bool {
	if udpOnlyProtocols[prof.Protocol] {
		return false
	}
	server, _ := resolveProbeTarget(prof)
	if server == "" || isLoopbackHost(server) {
		return false
	}
	return true
}

// tcpReachable measures a plain TCP connect to host:port, returning the latency
// in milliseconds or -1 when nothing accepted the connection.
//
// Deliberately NOT parser.TCPPing. That one demands a full TLS handshake with
// ServerName set to the server address whenever the port is 443 — the right test
// for a clean-IP scan, but the wrong one here. Most subscription profiles are
// `ip:443` Reality endpoints whose certificate is not issued for the IP literal,
// so the handshake fails and a healthy server is reported dead. That made
// auto-failover announce "nothing is reachable" for a working subscription.
//
// For "is this server up?" a completed TCP handshake is the whole answer; the
// protocol-specific proof comes later, from the connect attempt itself.
func tcpReachable(host string, port int, timeout time.Duration) int {
	return parser.TCPReachable(host, port, timeout)
}

// probeProfileTCP resolves a profile's server address and measures the TCP
// connect time to it, returning a latency of -1 when the profile carries no
// usable address or the connection did not complete.
//
// This is the one place that knows how to turn a profile into "an address worth
// knocking on", so the callers below cannot drift apart.
func probeProfileTCP(prof config.Profile, timeout time.Duration) (string, int, int) {
	server, port := resolveProbeTarget(prof)
	if server == "" {
		return "", 0, -1
	}
	return server, port, tcpReachable(server, port, timeout)
}

// FindFastestProfile concurrently tests responsive latency of profiles and returns the optimal node.
//
// Note the fallback: when nothing answers it still returns the first profile.
// That is deliberate here — this backs the "connect me to the fastest server"
// button, where trying something is better than refusing outright. Callers that
// must not act on an unreachable server (auto-failover) use
// ProbeReachableProfiles instead.
func FindFastestProfile(ctx context.Context, profiles []config.Profile, testURL string) (*config.Profile, int, error) {
	if len(profiles) == 0 {
		return nil, -1, fmt.Errorf("no profiles available for auto-selection")
	}

	if testURL == "" {
		testURL = "http://cp.cloudflare.com/generate_204"
	}

	type pingRes struct {
		profile config.Profile
		latency int
	}

	resChan := make(chan pingRes, len(profiles))
	var wg sync.WaitGroup

	sem := make(chan struct{}, 20) // concurrency limit

	for _, p := range profiles {
		if !CanTCPProbe(p) {
			continue
		}

		wg.Add(1)
		go func(prof config.Profile) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			_, _, latency := probeProfileTCP(prof, probeTimeout)
			if latency > 0 {
				resChan <- pingRes{profile: prof, latency: latency}
			}
		}(p)
	}

	wg.Wait()
	close(resChan)

	var valid []pingRes
	for r := range resChan {
		valid = append(valid, r)
	}

	if len(valid) == 0 {
		// Nothing answered, which for this button still means "try the most
		// plausible one" rather than "refuse" — just not one we have already
		// proven dead.
		fallback := firstOutsideCooldown(profiles)
		return &fallback, 999, nil
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].latency < valid[j].latency
	})

	best := valid[0].profile
	best.LatencyMs = valid[0].latency
	return &best, valid[0].latency, nil
}

// CountCoolingDown reports how many of the given profiles are inside a server
// failure cooldown.
//
// It exists so a failover that found nothing reachable can say *why*: "all six
// backups were dead" and "all six backups had already been written off two
// minutes ago" are different problems for the user, and without the number the
// second one reads like the first.
func CountCoolingDown(profiles []config.Profile) int {
	now := time.Now()
	n := 0
	for _, p := range profiles {
		if config.ProfileIsCoolingDown(p, now, 0) {
			n++
		}
	}
	return n
}

// firstOutsideCooldown returns the first profile that is not inside a server
// failure cooldown, falling back to the head of the list when every one of them
// is cooling down.
//
// It exists for the "nothing answered" guess paths, where refusing outright is
// worse than trying — but a server that was proven dead moments ago is the worst
// possible guess, and picking it is how one dead subscription host ended up
// retried ~80 times in two minutes.
func firstOutsideCooldown(profiles []config.Profile) config.Profile {
	now := time.Now()
	for _, p := range profiles {
		if !config.ProfileIsCoolingDown(p, now, 0) {
			return p
		}
	}
	if len(profiles) > 0 {
		return profiles[0]
	}
	return config.Profile{}
}

// RankFailoverCandidates orders backup profiles for a failover attempt:
//
//  1. profiles with a previously measured latency, fastest first;
//  2. profiles we have never measured, since an unmeasured server is a coin flip;
//  3. profiles with no address we can dial at all — an imported .ovpn, a
//     WireGuard file whose endpoint could not be parsed — last, because they can
//     never pass a probe and are only worth a blind attempt once everything
//     measurable has been ruled out.
//
// A profile inside its server failure cooldown is demoted below every healthy
// peer of the same bucket rather than dropped: a whole subscription can be
// cooling down after a brief ISP outage, and then the attempt is still worth
// making. Excluding is the wrong instruction; ordering is the right one.
//
// excludeID is the profile that just failed; retrying it is what the attempt
// counter exists to prevent. limit caps the list so a large subscription does
// not turn one failover into dozens of probes; pass 0 for no cap.
func RankFailoverCandidates(profiles []config.Profile, excludeID string, limit int) []config.Profile {
	now := time.Now()

	measured := make([]config.Profile, 0, len(profiles))
	unmeasured := make([]config.Profile, 0, len(profiles))
	unprobeable := make([]config.Profile, 0, len(profiles))
	cooling := make([]config.Profile, 0, len(profiles))

	for _, p := range profiles {
		if p.ID == excludeID {
			continue
		}
		if p.Server == "" && p.RawURI == "" && p.ConfigContent == "" {
			continue
		}
		if config.ProfileIsCoolingDown(p, now, 0) {
			cooling = append(cooling, p)
			continue
		}

		switch {
		case !CanTCPProbe(p):
			unprobeable = append(unprobeable, p)
		case p.LatencyMs > 0:
			measured = append(measured, p)
		default:
			unmeasured = append(unmeasured, p)
		}
	}

	sort.SliceStable(measured, func(i, j int) bool { return measured[i].LatencyMs < measured[j].LatencyMs })

	// The cooled-down tail keeps the same internal ordering as the healthy part,
	// so a cooldown only ever changes *when* a server is tried, never whether an
	// obviously unprobeable one is tried before a probeable one.
	sort.SliceStable(cooling, func(i, j int) bool {
		ci, cj := !CanTCPProbe(cooling[i]), !CanTCPProbe(cooling[j])
		if ci != cj {
			return !ci
		}
		li, lj := cooling[i].LatencyMs, cooling[j].LatencyMs
		if (li > 0) != (lj > 0) {
			return li > 0
		}
		if li > 0 && lj > 0 {
			return li < lj
		}
		return false
	})

	ranked := make([]config.Profile, 0, len(measured)+len(unmeasured)+len(unprobeable)+len(cooling))
	ranked = append(ranked, measured...)
	ranked = append(ranked, unmeasured...)
	ranked = append(ranked, unprobeable...)
	ranked = append(ranked, cooling...)

	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// UnprobeableCandidates returns the subset of candidates that a TCP probe cannot
// say anything about — no dialable address, a UDP-only endpoint, or a loopback
// address. An empty result means every candidate was testable.
func UnprobeableCandidates(candidates []config.Profile) []config.Profile {
	out := make([]config.Profile, 0, len(candidates))
	for _, p := range candidates {
		if !CanTCPProbe(p) {
			out = append(out, p)
		}
	}
	return out
}

// ProbeReachableProfiles concurrently probes every profile and returns only the
// ones that actually answered, fastest first. An empty result means nothing
// responded — it is never silently replaced with a best guess.
//
// This exists for auto-failover. The TUN crash-loop incident had failover spend
// every attempt moving between servers while the real fault was a local adapter,
// and the final target (WARP/WireGuard) was independently dead — so the loop
// never had a working path and the user watched a spinner instead of being told
// anything. A dead backup is worse than no backup: it burns the attempt budget
// and hides the real cause. Probing first lets the caller say "nothing is
// reachable" out loud.
func ProbeReachableProfiles(ctx context.Context, profiles []config.Profile) []AutoSelectResult {
	if len(profiles) == 0 {
		return nil
	}

	type pingRes struct {
		profile config.Profile
		latency int
	}

	resChan := make(chan pingRes, len(profiles))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // same concurrency ceiling as FindFastestProfile

	for _, p := range profiles {
		if !CanTCPProbe(p) {
			continue
		}

		wg.Add(1)
		go func(prof config.Profile) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			_, _, latency := probeProfileTCP(prof, probeTimeout)
			if latency > 0 {
				resChan <- pingRes{profile: prof, latency: latency}
			}
		}(p)
	}

	wg.Wait()
	close(resChan)

	var reachable []AutoSelectResult
	for r := range resChan {
		prof := r.profile
		prof.LatencyMs = r.latency
		reachable = append(reachable, AutoSelectResult{Profile: &prof, LatencyMs: r.latency})
	}

	sort.Slice(reachable, func(i, j int) bool {
		return reachable[i].LatencyMs < reachable[j].LatencyMs
	})

	return reachable
}
