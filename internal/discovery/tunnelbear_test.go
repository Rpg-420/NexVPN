package discovery

import (
	"strings"
	"testing"
)

func TestTunnelBearCatalogueIsWellFormed(t *testing.T) {
	list := ListTunnelBearCountries()
	if len(list) != 47 {
		t.Fatalf("expected the operator's 47 published endpoints, got %d", len(list))
	}

	seen := make(map[string]bool, len(list))
	for _, c := range list {
		if c.ID == "" || c.Name == "" || c.NameFA == "" || c.Host == "" {
			t.Errorf("incomplete entry: %+v", c)
		}
		if !strings.HasSuffix(c.Host, TunnelBearHostSuffix) {
			t.Errorf("%s: host %q is outside the operator's domain", c.ID, c.Host)
		}
		if c.Code != "" && len(c.Code) != 2 {
			t.Errorf("%s: %q is not an ISO 3166-1 alpha-2 code", c.ID, c.Code)
		}
		if seen[c.ID] {
			t.Errorf("duplicate country id %q", c.ID)
		}
		seen[c.ID] = true
	}
}

// The catalogue is a defensive copy: a caller that sorts or trims what it gets
// must not be able to reorder the package's own list.
func TestListTunnelBearCountriesIsACopy(t *testing.T) {
	first := ListTunnelBearCountries()
	original := first[0].Host

	first[0].Host = "mutated.example.com"

	second := ListTunnelBearCountries()
	if second[0].Host != original {
		t.Fatalf("the returned slice aliases the catalogue: got %q, want %q", second[0].Host, original)
	}
}

func TestIsTunnelBearHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"de.lazerpenguin.com", true},
		{"us-east.lazerpenguin.com", true},
		{"DE.LAZERPENGUIN.COM", true},
		{"  de.lazerpenguin.com  ", true},
		{"lazerpenguin.com", false},
		{"notlazerpenguin.com", false},
		{"evil-lazerpenguin.com", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsTunnelBearHost(tc.host); got != tc.want {
			t.Errorf("IsTunnelBearHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

// A host that belongs to the operator but is not in the catalogue — one the
// user imported by hand, or one the operator added after this build — must
// still produce a usable profile rather than being rejected.
func TestCountryForHostSynthesizesUnknownHosts(t *testing.T) {
	known := CountryForHost("de.lazerpenguin.com")
	if known.Name != "Germany" {
		t.Errorf("a catalogue host should keep its entry, got %q", known.Name)
	}

	unknown := CountryForHost("zz.lazerpenguin.com")
	if unknown.Host != "zz.lazerpenguin.com" {
		t.Errorf("host not preserved: %q", unknown.Host)
	}
	if unknown.Code != "ZZ" {
		t.Errorf("expected a ZZ flag code derived from the host, got %q", unknown.Code)
	}
	if unknown.NameFA == "" {
		t.Error("a synthesized entry still needs a label to render")
	}
}

// NexVPN's OpenVPN adapter writes the profile to a temp file and runs
// `openvpn --config` on it, so a relative `ca <file>.crt` reference can never
// resolve. The certificate has to be in the text.
func TestBuildTunnelBearConfigInlinesTheCertificate(t *testing.T) {
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)

	if strings.Contains(cfg, "openvpn-server-ca.crt") {
		t.Error("the config still references an external certificate file")
	}
	if !strings.Contains(cfg, "<ca>\n") || !strings.Contains(cfg, "\n</ca>") {
		t.Error("the CA is not wrapped in a <ca> block")
	}
	if !strings.Contains(cfg, "-----BEGIN CERTIFICATE-----") ||
		!strings.Contains(cfg, "-----END CERTIFICATE-----") {
		t.Error("the embedded CA is not a complete PEM certificate")
	}
	// The certificate body itself, not just its delimiters.
	if !strings.Contains(cfg, TunnelBearCA) {
		t.Error("the embedded CA does not match TunnelBearCA")
	}
}

// The operator names the cipher only in `data-ciphers`; emitting `cipher` too
// costs nothing and removes the ambiguity for anything reading the profile.
func TestBuildTunnelBearConfigNamesTheCipherTwice(t *testing.T) {
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)

	if !strings.Contains(cfg, "data-ciphers AES-256-CBC") {
		t.Error("data-ciphers is missing")
	}
	if !strings.Contains(cfg, "cipher AES-256-CBC") {
		t.Error("the plain `cipher` directive is missing")
	}
	if !strings.Contains(cfg, "auth SHA256") {
		t.Error("auth is missing")
	}
	if !strings.Contains(cfg, "auth-user-pass") {
		t.Error("the profile must ask for credentials")
	}
}

func TestBuildTunnelBearConfigDirectForm(t *testing.T) {
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)

	if !strings.Contains(cfg, "proto udp") {
		t.Error("the direct form should use the operator's published UDP transport")
	}
	if !strings.Contains(cfg, "remote de.lazerpenguin.com 443") {
		t.Error("the direct form should name the endpoint and the published port")
	}
	if strings.Contains(cfg, "tcp-client") {
		t.Error("the direct form must not mention TCP")
	}
	if strings.Contains(cfg, "net_gateway") {
		t.Error("a direct profile has no relay socket to protect")
	}
}

func TestBuildTunnelBearConfigRelayForm(t *testing.T) {
	ips := []string{"203.0.113.7", "203.0.113.8"}
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 41234, ips)

	if !strings.Contains(cfg, "proto tcp-client") {
		t.Error("the relay form must use TCP")
	}
	if !strings.Contains(cfg, "remote 127.0.0.1 41234 tcp-client") {
		t.Error("the relay form must dial the loopback relay")
	}
	if strings.Contains(cfg, "proto udp") {
		t.Error("the relay form must not also carry a UDP remote")
	}
	if strings.Contains(cfg, "explicit-exit-notify") {
		t.Error("explicit-exit-notify is UDP-only and OpenVPN rejects the config when it sits beside a TCP remote")
	}

	// Without these the relay's own socket to the server is captured by the
	// redirect-gateway route the tunnel installs, and the connection deadlocks.
	for _, ip := range ips {
		want := "route " + ip + " 255.255.255.255 net_gateway"
		if !strings.Contains(cfg, want) {
			t.Errorf("missing host route for the relay address: %q", want)
		}
	}
}

func TestBuildTunnelBearConfigIgnoresBlankRelayAddresses(t *testing.T) {
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 41234, []string{"", "  ", "203.0.113.9"})
	if strings.Contains(cfg, "route  255.255.255.255") {
		t.Error("a blank address produced a malformed route line")
	}
	if !strings.Contains(cfg, "route 203.0.113.9 255.255.255.255 net_gateway") {
		t.Error("the real address was dropped along with the blanks")
	}
}

// The adapter chooses the tunnel device; a pinned name is what its sanitizer
// strips from imported profiles, so the built config must not carry one.
func TestBuildTunnelBearConfigDoesNotPinADeviceName(t *testing.T) {
	cfg := BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)
	if strings.Contains(cfg, "dev tun0") {
		t.Error("the config pins a device name; the adapter owns that choice")
	}
	if !strings.Contains(cfg, "dev tun\n") {
		t.Error("the config should still declare a tun device")
	}
}

func TestLookupTunnelBearCountry(t *testing.T) {
	if _, ok := LookupTunnelBearCountry("de"); !ok {
		t.Error("de should be in the catalogue")
	}
	if _, ok := LookupTunnelBearCountry("nope"); ok {
		t.Error("an unknown id should not resolve")
	}
	if _, ok := LookupTunnelBearCountryByHost("de.lazerpenguin.com"); !ok {
		t.Error("de.lazerpenguin.com should resolve to its entry")
	}
}
