package parser

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nexvpn/internal/config"
)

// Resolver policy for a Layer-3 primary, and the reason it is not "whatever the
// user configured".
//
// The 2026-10-12 log has ~200 lines of
//
//	dns: lookup failed for X: (exchange4: read tcp 192.168.1.103:xxxxx->1.1.1.1:443:
//	wsarecv: An existing connection was forcibly closed ...)
//
// — every query of a WireGuard session, answered by a TCP reset from the network.
// The configured resolver is DoH (`https://1.1.1.1/dns-query`, the store
// default), and the machine resolves names through its router (192.168.1.1)
// perfectly well with no tunnel up. sing-box 1.14 offers no fallback chain —
// `dns.final` and `dns.rules[].server` both reject a list, verified against the
// bundled core — so the app has to *choose* the resolver that survives, and for a
// Layer-3 primary that choice cannot be the one being reset.
//
// `TestSingBoxWireGuardEngineStartsWithoutDNSFatal` in dns_routing_deadlock_test.go
// covers the other half: that the generated config actually boots.

// settingsWithDNS mirrors what the store hands the builder: a DoH primary and a
// plain secondary — the combination the 2026-10-12 log ran with.
func settingsWithDNS() config.Settings {
	return config.Settings{
		ProxyMode:   "system_proxy",
		RoutingMode: "bypass_iran",
		DNSServer:   "https://1.1.1.1/dns-query",
		DNSFallback: "8.8.8.8",
	}
}

// layerThreeConfig returns a generated WireGuard config whose endpoint is a
// *hostname*, so the bootstrap rule exists to be asserted on.
func layerThreeConfig(t *testing.T) map[string]interface{} {
	t.Helper()

	outbound, _, err := ParseWireGuardConf(testWireGuardConf)
	if err != nil {
		t.Fatalf("ParseWireGuardConf: %v", err)
	}
	raw, err := BuildSingBoxConfigAdvanced(outbound, "system_proxy", "bypass_iran", "wg-endpoint.example.net", settingsWithDNS())
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced: %v", err)
	}
	return decodeConfig(t, raw)
}

func proxyPrimaryConfig(t *testing.T) map[string]interface{} {
	t.Helper()

	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@203.0.113.9:443?security=tls&sni=example.com&type=ws&path=%2F#Node"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI: %v", err)
	}
	raw, err := BuildSingBoxConfigAdvanced(outbound, "system_proxy", "bypass_iran", "edge.example.com", settingsWithDNS())
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced: %v", err)
	}
	return decodeConfig(t, raw)
}

func dnsBlock(t *testing.T, cfg map[string]interface{}) map[string]interface{} {
	t.Helper()
	dns, ok := cfg["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("config has no dns block")
	}
	return dns
}

// A Layer-3 session must resolve through the machine, not through the reset
// endpoint, and the DNS router and the route's dial resolver must agree — two
// different answers is what produced a failure every connection paid for.
func TestLayerThreeResolvesOutsideTheResetEndpoint(t *testing.T) {
	cfg := layerThreeConfig(t)
	dns := dnsBlock(t, cfg)
	route := cfg["route"].(map[string]interface{})

	if dns["final"] != "dns-direct" {
		t.Errorf("dns.final = %v, want dns-direct: the configured DoH server is being reset on this class of network", dns["final"])
	}
	if route["default_domain_resolver"] != "dns-direct" {
		t.Errorf("route.default_domain_resolver = %v, want dns-direct so dial-time and DNS-router resolution cannot disagree", route["default_domain_resolver"])
	}
}

// The user's second resolver has to be reachable from somewhere. It used to be
// written into every generated config and referenced by nothing — a settings
// field with no effect, which is worse than an absent one.
func TestLayerThreeBootstrapUsesTheSecondResolver(t *testing.T) {
	cfg := layerThreeConfig(t)
	dns := dnsBlock(t, cfg)

	rules, err := json.Marshal(dns["rules"])
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	if !strings.Contains(string(rules), "wg-endpoint.example.net") {
		t.Fatalf("no bootstrap rule for the tunnel endpoint:\n%s", rules)
	}
	if !strings.Contains(string(rules), "dns-fallback") {
		t.Errorf("dns-fallback is defined but nothing references it; the user's second resolver is dead config:\n%s", rules)
	}
	// A poisoned router resolver must not be able to hand back a blocked address
	// for the endpoint itself.
	if strings.Contains(string(rules), `"domain":["wg-endpoint.example.net"],"server":"dns-direct"`) {
		t.Errorf("the endpoint bootstrap should not depend on the local resolver")
	}
}

// A proxy primary keeps its resolver inside the tunnel: the domain is sent to the
// server anyway, so resolving it locally would only add a leak.
func TestProxyPrimaryKeepsTheTunnelResolver(t *testing.T) {
	cfg := proxyPrimaryConfig(t)
	dns := dnsBlock(t, cfg)

	if dns["final"] != "dns-remote" {
		t.Errorf("dns.final = %v, want dns-remote for a proxy primary", dns["final"])
	}
	srv := dnsServer(t, cfg, "dns-remote")
	if srv == nil {
		t.Fatalf("dns-remote missing")
	}
	if srv["detour"] != "proxy" {
		t.Errorf("dns-remote detour = %v, want proxy", srv["detour"])
	}
}

// The detour the Layer-3 config may *not* carry: naming the bare `direct`
// outbound is a startup FATAL in sing-box 1.12+ (see the runtime test), and
// omitting the key is the supported way to say "dial straight out".
func TestLayerThreeDNSCarriesNoDirectDetour(t *testing.T) {
	cfg := layerThreeConfig(t)
	dns := dnsBlock(t, cfg)

	servers, _ := dns["servers"].([]interface{})
	for _, raw := range servers {
		s, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		tag, _ := s["tag"].(string)
		if (tag == "dns-remote" || tag == "dns-fallback") && s["detour"] == "direct" {
			t.Errorf("%s has detour=direct: the bundled core refuses to start an https/udp server detoured at an empty direct outbound", tag)
		}
	}
}

// startGeneratedConfig boots the bundled core with a TUN-less copy of a generated
// config on relocated ports, and reports whether it stayed up through service
// initialization.
func startGeneratedConfig(t *testing.T, cfg map[string]interface{}) (string, bool) {
	t.Helper()

	sb := testSingBoxBinary(t)
	if sb == "" {
		t.Skip("sing-box binary not found, skipping core runtime validation")
	}

	probe := relocatePorts(cfg)
	probe["inbounds"] = []interface{}{
		map[string]interface{}{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": freeTCPPort(t)},
	}
	if exp, ok := probe["experimental"].(map[string]interface{}); ok {
		delete(exp, "clash_api")
	}

	data, err := json.MarshalIndent(probe, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	tmp, err := os.CreateTemp("", "sbpolicy-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		t.Fatalf("write: %v", err)
	}
	tmp.Close()

	binDir, _ := filepath.Abs(filepath.Dir(sb))
	cmd := exec.Command(sb, "run", "-c", tmp.Name(), "-D", binDir)
	// The same compatibility flags the adapter exports at run time.
	cmd.Env = append(os.Environ(),
		"ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true",
		"ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true",
		"ENABLE_DEPRECATED_WIREGUARD_OUTBOUND=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true",
		"ENABLE_DEPRECATED_LEGACY_DNS_FAKEIP_OPTIONS=true",
		"ENABLE_DEPRECATED_FAKEIP=true",
		"ENABLE_DEPRECATED_LEGACY_INBOUND=true",
		"ENABLE_DEPRECATED_INBOUND=true",
		"ENABLE_DEPRECATED_TUN=true",
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return out.String(), false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		return out.String(), false
	case <-time.After(2500 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	<-done
	return out.String(), true
}

// The policy above is only worth anything if the core accepts the result. Both
// generated shapes are booted, because `sing-box check` passes configs the
// service then refuses.
func TestGeneratedResolverPoliciesBoot(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]interface{}
	}{
		{name: "wireguard (layer-3)", cfg: layerThreeConfig(t)},
		{name: "vless (proxy)", cfg: proxyPrimaryConfig(t)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, started := startGeneratedConfig(t, tc.cfg)
			if !started {
				t.Fatalf("the bundled core refused the generated config:\n%s", out)
			}
		})
	}
}

// relocatePorts moves the well-known listeners aside on a copy of the config, so
// a probe never collides with a running NexVPN — a port clash kills the process
// inside the same window the test measures and would read as a rejected config.
func relocatePorts(cfg map[string]interface{}) map[string]interface{} {
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return cfg
	}

	if inbounds, ok := out["inbounds"].([]interface{}); ok {
		for _, raw := range inbounds {
			in, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if p, ok := in["listen_port"].(float64); ok && p >= 10800 && p < 11000 {
				in["listen_port"] = 20000 + int(p) - 10800
			}
		}
	}
	return out
}
