package parser

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// singBoxCompatEnv mirrors the compatibility flags the adapter exports when it
// launches the core (internal/protocols/singbox.go). Validating a config without
// them would test a core the app never runs.
var singBoxCompatEnv = []string{
	"ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true",
	"ENABLE_DEPRECATED_LEGACY_DNS_FAKEIP_OPTIONS=true",
	"ENABLE_DEPRECATED_FAKEIP=true",
	"ENABLE_DEPRECATED_WIREGUARD_OUTBOUND=true",
	"ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true",
	"ENABLE_DEPRECATED_LEGACY_INBOUND=true",
	"ENABLE_DEPRECATED_INBOUND=true",
	"ENABLE_DEPRECATED_TUN=true",
	"ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true",
}

// These tests pin the DNS topology of the generated sing-box config.
//
// The 2026-10-11 log is the reason: a WireGuard engine asked every domain of
// every connection through a resolver that was itself routed through that
// WireGuard tunnel. The tunnel could not carry the query, the query timed out
// after 10s, and nothing ever connected — ~90 identical failures in two
// minutes, with no error a user could act on. A Layer-3 primary must resolve
// outside the tunnel; a proxy primary must not, because resolving outside it
// would leak the names it is supposed to hide.

func testSingBoxBinary(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Join("..", "..", "bin", "sing-box.exe"),
		filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe"),
		filepath.Join("..", "..", "bin", "sing-box"),
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if fi, statErr := os.Stat(abs); statErr == nil && !fi.IsDir() {
			return abs
		}
	}
	return ""
}

func decodeConfig(t *testing.T, cfg string) map[string]interface{} {
	t.Helper()

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(cfg), &out); err != nil {
		t.Fatalf("generated config is not valid JSON: %v\n%s", err, cfg)
	}
	return out
}

// dnsServer returns the server entry with the given tag, or nil.
func dnsServer(t *testing.T, cfg map[string]interface{}, tag string) map[string]interface{} {
	t.Helper()

	dnsBlock, ok := cfg["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("config has no dns block: %v", cfg)
	}
	servers, ok := dnsBlock["servers"].([]interface{})
	if !ok {
		t.Fatalf("dns.servers is not a list: %T", dnsBlock["servers"])
	}
	for _, raw := range servers {
		srv, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if srv["tag"] == tag {
			return srv
		}
	}
	return nil
}

const testWireGuardConf = `
[Interface]
PrivateKey = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
Address = 10.0.0.2/32
DNS = 1.1.1.1

[Peer]
PublicKey = bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb=
Endpoint = 198.51.100.1:51820
AllowedIPs = 0.0.0.0/0
`

func TestSingBoxDNSWireGuardResolvesOutsideTunnel(t *testing.T) {
	outbound, host, err := ParseWireGuardConf(testWireGuardConf)
	if err != nil {
		t.Fatalf("ParseWireGuardConf failed: %v", err)
	}
	if outbound["type"] != "wireguard" {
		t.Fatalf("expected a wireguard outbound, got %v", outbound["type"])
	}

	cfgStr, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}
	cfg := decodeConfig(t, cfgStr)

	for _, tag := range []string{"dns-remote", "dns-fallback"} {
		srv := dnsServer(t, cfg, tag)
		if srv == nil {
			t.Fatalf("%s is missing from the generated config", tag)
		}
		// A Layer-3 primary must resolve outside the tunnel, but it must do so by
		// *omitting* the detour. Since sing-box 1.12 a DNS server with no detour
		// dials directly ("equivalent to using an empty direct outbound"), while
		// naming the bare `direct` outbound is a startup FATAL —
		// "detour to an empty direct outbound makes no sense" (sing-box #2803).
		if _, ok := srv["detour"]; ok {
			t.Errorf("%s detour = %v, want it absent: naming the empty `direct` outbound makes sing-box 1.14 refuse to start (the WireGuard-engine FATAL of the 2026-10-12 log)",
				tag, srv["detour"])
		}
	}
}

func TestSingBoxDNSProxyPrimaryKeepsTunnelResolver(t *testing.T) {
	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@203.0.113.9:443?security=tls&sni=example.com&type=ws&path=%2F#Node"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	cfgStr, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}
	cfg := decodeConfig(t, cfgStr)

	srv := dnsServer(t, cfg, "dns-remote")
	if srv == nil {
		t.Fatalf("dns-remote is missing from the generated config")
	}
	// A proxy-style outbound is given the domain and resolves it remotely, so
	// routing its own DNS through the tunnel leaks nothing. Moving it to direct
	// would expose every query name to the local network.
	if srv["detour"] != "proxy" {
		t.Errorf("dns-remote detour = %v, want proxy (the resolver of a proxy outbound must stay inside the tunnel)", srv["detour"])
	}
}

func TestSingBoxDNSQueryTimeoutIsBounded(t *testing.T) {
	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@203.0.113.9:443?security=tls&sni=example.com&type=ws&path=%2F#Node"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	cfgStr, err := BuildSingBoxConfig(outbound, "tun", "global", "203.0.113.9")
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}
	cfg := decodeConfig(t, cfgStr)

	dnsBlock, _ := cfg["dns"].(map[string]interface{})
	if dnsBlock["timeout"] != dnsQueryTimeout {
		t.Errorf("dns.timeout = %v, want %s: without a bound an unreachable resolver costs the core's 10s default per lookup and per connection",
			dnsBlock["timeout"], dnsQueryTimeout)
	}

	// The field is a `dns` option in sing-box 1.14. Repeating it per server is a
	// schema error that kills the core at startup, so it must not appear there.
	if srv := dnsServer(t, cfg, "dns-remote"); srv != nil {
		if _, ok := srv["timeout"]; ok {
			t.Errorf("dns-remote carries a per-server timeout, which sing-box 1.14 rejects with `unknown field`")
		}
	}
}

// TestSingBoxDNSConfigPassesCoreCheck runs the real bundled core over both DNS
// topologies. The schema claims above are only worth what the binary will pay
// for them: `timeout` and `detour` are accepted at the `dns` level and rejected
// inside a server object, and that is exactly the kind of thing a reviewer
// cannot verify from the source alone.
func TestSingBoxDNSConfigPassesCoreCheck(t *testing.T) {
	sbBin := testSingBoxBinary(t)
	if sbBin == "" {
		t.Skip("sing-box binary not found, skipping core validation")
	}

	wgOutbound, wgHost, err := ParseWireGuardConf(testWireGuardConf)
	if err != nil {
		t.Fatalf("ParseWireGuardConf failed: %v", err)
	}
	vlessOutbound, err := ParseOutboundFromURI("vless://9cf8b982-263a-44d5-a8c6-111111111111@203.0.113.9:443?security=tls&sni=example.com&type=ws&path=%2F#Node")
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	cases := []struct {
		name     string
		outbound map[string]interface{}
		host     string
	}{
		{name: "wireguard (layer-3)", outbound: wgOutbound, host: wgHost},
		{name: "vless (proxy)", outbound: vlessOutbound, host: "203.0.113.9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgStr, err := BuildSingBoxConfig(tc.outbound, "tun", "bypass_iran", tc.host)
			if err != nil {
				t.Fatalf("BuildSingBoxConfig failed: %v", err)
			}

			tmp, err := os.CreateTemp("", "sb-dns-topology-*.json")
			if err != nil {
				t.Fatalf("CreateTemp failed: %v", err)
			}
			defer func() { _ = os.Remove(tmp.Name()) }()
			if _, err := tmp.WriteString(cfgStr); err != nil {
				_ = tmp.Close()
				t.Fatalf("writing temp config failed: %v", err)
			}
			_ = tmp.Close()

			binDir, _ := filepath.Abs(filepath.Dir(sbBin))
			cmd := exec.Command(sbBin, "check", "-c", tmp.Name(), "-D", binDir)
			// The same compatibility flags the adapter exports at run time, so the
			// check validates the config the core will actually be handed.
			cmd.Env = append(os.Environ(), singBoxCompatEnv...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sing-box check rejected the generated config: %v\nOutput: %s\nConfig:\n%s",
					err, strings.TrimSpace(string(out)), cfgStr)
			}
		})
	}
}

// freeTCPPort returns a loopback port that is free right now, so the test's
// throwaway inbound never collides with a running NexVPN instance.
func freeTCPPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port probe failed: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// TestSingBoxWireGuardEngineStartsWithoutDNSFatal guards the one failure the
// `check`-based tests above cannot see.
//
// "detour to an empty direct outbound makes no sense" is raised when the DNS
// service *starts*, not when the config is parsed: `sing-box check` happily
// accepts the offending config, which is precisely why the WireGuard engine
// shipped broken and every attempt in the 2026-10-12 log died at
// `FATAL[0000] start service: start dns/https[dns-remote]` — TUN and the
// System-Proxy fallback alike. This boots the real core over a TUN-less copy of
// the generated config and fails if it refuses to come up.
func TestSingBoxWireGuardEngineStartsWithoutDNSFatal(t *testing.T) {
	sbBin := testSingBoxBinary(t)
	if sbBin == "" {
		t.Skip("sing-box binary not found, skipping core runtime validation")
	}

	wgOutbound, wgHost, err := ParseWireGuardConf(testWireGuardConf)
	if err != nil {
		t.Fatalf("ParseWireGuardConf failed: %v", err)
	}
	cfgStr, err := BuildSingBoxConfig(wgOutbound, "tun", "bypass_iran", wgHost)
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}

	// Strip everything that touches the host: the TUN inbound would rewrite the
	// routing table, the clash API and the fixed proxy ports could collide with a
	// running instance, and the tun-in DNS rule would reference a missing
	// inbound. None of it is what this test is about.
	cfg["inbounds"] = []interface{}{
		map[string]interface{}{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": freeTCPPort(t)},
	}
	delete(cfg, "experimental")
	cfg["log"] = map[string]interface{}{"level": "info"}
	if dnsBlock, ok := cfg["dns"].(map[string]interface{}); ok {
		dnsBlock["rules"] = []interface{}{}
	}
	safeCfg, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}

	tmp, err := os.CreateTemp("", "sb-wg-start-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(safeCfg); err != nil {
		_ = tmp.Close()
		t.Fatalf("writing temp config failed: %v", err)
	}
	tmp.Close()

	binDir, _ := filepath.Abs(filepath.Dir(sbBin))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, sbBin, "run", "-c", tmp.Name(), "-D", binDir)
	cmd.Env = append(os.Environ(), singBoxCompatEnv...)
	out, _ := cmd.CombinedOutput()
	text := string(out)

	if strings.Contains(text, "empty direct outbound") {
		t.Fatalf("sing-box refused to start: the DNS detour points at an empty direct outbound (sing-box #2803)\nOutput:\n%s", text)
	}
	if strings.Contains(text, "FATAL") {
		t.Fatalf("sing-box died during startup on the WireGuard config:\n%s", text)
	}
	if !strings.Contains(text, "sing-box started") {
		t.Fatalf("sing-box never reported a successful start on the WireGuard config:\n%s", text)
	}
}
