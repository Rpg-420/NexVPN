package parser

import (
	"strings"
	"testing"
)

// amneziaTestParams returns a fully-populated parameter set used across the
// AmneziaWG tests. Every value is distinct on purpose: a mix-up between two
// keys (e.g. Jmin/Jmax or H1/H2) would then fail the assertions instead of
// silently passing.
func amneziaTestParams() AmneziaParams {
	return AmneziaParams{
		Jc:   4,
		Jmin: 40,
		Jmax: 70,
		S1:   50,
		S2:   60,
		H1:   1111111,
		H2:   2222222,
		H3:   3333333,
		H4:   4444444,
	}
}

func TestAmneziaParamsIsZero(t *testing.T) {
	if !(AmneziaParams{}).IsZero() {
		t.Fatal("zero-value AmneziaParams must report IsZero() == true")
	}
	if amneziaTestParams().IsZero() {
		t.Fatal("fully-populated AmneziaParams must report IsZero() == false")
	}
	// A single non-zero field is enough to make the whole struct non-zero.
	if (AmneziaParams{H4: 1}).IsZero() {
		t.Fatal("AmneziaParams{H4:1} must report IsZero() == false")
	}
}

func TestAmneziaINILines(t *testing.T) {
	got := amneziaTestParams().amneziaINILines()
	for _, want := range []string{
		"Jc = 4", "Jmin = 40", "Jmax = 70",
		"S1 = 50", "S2 = 60",
		"H1 = 1111111", "H2 = 2222222", "H3 = 3333333", "H4 = 4444444",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("amneziaINILines() missing %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "= 0") {
		t.Errorf("amneziaINILines() must omit zero-valued keys; got:\n%s", got)
	}
	if (AmneziaParams{}).amneziaINILines() != "" {
		t.Error("zero-value AmneziaParams must render no INI lines")
	}
}

func TestAmneziaURIQuery(t *testing.T) {
	got := amneziaTestParams().amneziaURIQuery()
	for _, want := range []string{
		"&jc=4", "&jmin=40", "&jmax=70",
		"&s1=50", "&s2=60",
		"&h1=1111111", "&h2=2222222", "&h3=3333333", "&h4=4444444",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("amneziaURIQuery() missing %q; got: %s", want, got)
		}
	}
	if (AmneziaParams{}).amneziaURIQuery() != "" {
		t.Error("zero-value AmneziaParams must render an empty URI query")
	}
}

// TestParseWireGuardConfAmnezia verifies the parser reads every AmneziaWG
// obfuscation key out of an INI and surfaces it on the sing-box outbound under
// the canonical `junk_packet_*` / `*_packet_junk_size` / `*_magic_header` names.
// This test is fully offline.
func TestParseWireGuardConfAmnezia(t *testing.T) {
	conf := `[Interface]
PrivateKey = aGVsbG8gd29ybGQ=
Address = 172.16.0.2/32, 2606:4700:110::1/128
DNS = 1.1.1.1
MTU = 1280
Jc = 4
Jmin = 40
Jmax = 70
S1 = 50
S2 = 60
H1 = 1111111
H2 = 2222222
H3 = 3333333
H4 = 4444444

[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 8.6.112.31:1180
PersistentKeepalive = 25`

	outbound, _, err := ParseWireGuardConf(conf)
	if err != nil {
		t.Fatalf("ParseWireGuardConf failed: %v", err)
	}

	want := map[string]int{
		"junk_packet_count":             4,
		"junk_packet_min_size":          40,
		"junk_packet_max_size":          70,
		"init_packet_junk_size":         50,
		"response_packet_junk_size":     60,
		"init_packet_magic_header":      1111111,
		"response_packet_magic_header":  2222222,
		"underload_packet_magic_header": 3333333,
		"transport_header_magic_header": 4444444,
	}
	for key, expected := range want {
		raw, ok := outbound[key]
		if !ok {
			t.Errorf("outbound is missing %q", key)
			continue
		}
		got, ok := asInt(raw)
		if !ok {
			t.Errorf("%s: unexpected type %T (%v)", key, raw, raw)
			continue
		}
		if got != expected {
			t.Errorf("%s: want %d, got %d", key, expected, got)
		}
	}
}

// TestGenerateCloudflareWarpWithAmneziaRoundTrip exercises the full
// generate -> parse path. It performs a best-effort Cloudflare registration
// over the network, so it is skipped under `go test -short`.
func TestGenerateCloudflareWarpWithAmneziaRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent WARP registration test in -short mode")
	}

	profile, err := GenerateCloudflareWarpWithAmnezia("8.6.112.31", 1180, amneziaTestParams())
	if err != nil {
		t.Fatalf("GenerateCloudflareWarpWithAmnezia failed: %v", err)
	}
	if profile == nil {
		t.Fatal("expected a non-nil profile")
	}

	// 1. The obfuscation keys must land inside [Interface], i.e. before [Peer].
	peerIdx := strings.Index(profile.ConfigContent, "\n\n[Peer]")
	if peerIdx < 0 {
		t.Fatalf("generated INI has no [Peer] section:\n%s", profile.ConfigContent)
	}
	interfacePart := profile.ConfigContent[:peerIdx]
	for _, want := range []string{"Jc = 4", "Jmin = 40", "H1 = 1111111", "H4 = 4444444"} {
		if !strings.Contains(interfacePart, want) {
			t.Errorf("[Interface] section missing %q:\n%s", want, interfacePart)
		}
	}

	// 2. The wireguard:// URI must carry the same parameters before its fragment.
	hashIdx := strings.Index(profile.RawURI, "#")
	if hashIdx < 0 {
		t.Fatalf("generated URI has no fragment: %s", profile.RawURI)
	}
	queryPart := profile.RawURI[:hashIdx]
	for _, want := range []string{"&jc=4", "&jmin=40", "&h1=1111111", "&h4=4444444"} {
		if !strings.Contains(queryPart, want) {
			t.Errorf("URI query missing %q: %s", want, queryPart)
		}
	}

	// 3. Round-trip: parsing the generated INI must recover every parameter.
	outbound, _, err := ParseWireGuardConf(profile.ConfigContent)
	if err != nil {
		t.Fatalf("round-trip ParseWireGuardConf failed: %v", err)
	}
	checks := map[string]int{
		"junk_packet_count":             4,
		"junk_packet_min_size":          40,
		"junk_packet_max_size":          70,
		"init_packet_junk_size":         50,
		"response_packet_junk_size":     60,
		"init_packet_magic_header":      1111111,
		"response_packet_magic_header":  2222222,
		"underload_packet_magic_header": 3333333,
		"transport_header_magic_header": 4444444,
	}
	for key, expected := range checks {
		v, ok := outbound[key]
		if !ok {
			t.Errorf("round-trip lost %q", key)
			continue
		}
		got, ok := asInt(v)
		if !ok {
			t.Errorf("%s: unexpected type %T", key, v)
			continue
		}
		if got != expected {
			t.Errorf("round-trip %s: want %d, got %d", key, expected, got)
		}
	}
}

// TestGenerateCloudflareWarpWithAmneziaZero ensures a zero-value parameter set
// is a true no-op: no AmneziaWG keys injected and the plain description kept.
func TestGenerateCloudflareWarpWithAmneziaZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent WARP registration test in -short mode")
	}

	profile, err := GenerateCloudflareWarpWithAmnezia("8.6.112.31", 1180, AmneziaParams{})
	if err != nil {
		t.Fatalf("GenerateCloudflareWarpWithAmnezia failed: %v", err)
	}
	if strings.Contains(profile.ConfigContent, "Jc =") {
		t.Errorf("zero params must not inject AmneziaWG INI keys:\n%s", profile.ConfigContent)
	}
	if strings.Contains(profile.RawURI, "&jc=") {
		t.Errorf("zero params must not inject AmneziaWG URI params: %s", profile.RawURI)
	}
	if profile.Description != "Optimized Cloudflare WARP with Clean IP" {
		t.Errorf("zero params must keep the plain description, got %q", profile.Description)
	}
}

// asInt normalises the integer types the parser may store (int, uint32, int64)
// so the assertions above do not have to care which one was used.
func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	default:
		return 0, false
	}
}
