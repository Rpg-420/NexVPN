package main

import (
	"strings"
	"testing"

	"nexvpn/internal/config"
	"nexvpn/internal/discovery"
)

// tunnelBearHostOf is the gate that decides whether a profile gets the saved
// account and the loopback relay. A false positive would sign a profile in with
// a TunnelBear password while it dialled somebody else's server, so the
// negative cases carry as much weight as the positive ones.
func TestTunnelBearHostOf(t *testing.T) {
	tests := []struct {
		name    string
		profile config.Profile
		want    string
	}{
		{
			name:    "reads the server field",
			profile: config.Profile{Server: "de.lazerpenguin.com"},
			want:    "de.lazerpenguin.com",
		},
		{
			name: "reads the config text when the server field is empty",
			profile: config.Profile{
				ConfigContent: "client\ndev tun\nremote nl.lazerpenguin.com 443 udp\n",
			},
			want: "nl.lazerpenguin.com",
		},
		{
			name: "the server field wins over the config text",
			profile: config.Profile{
				Server:        "jp.lazerpenguin.com",
				ConfigContent: "remote nl.lazerpenguin.com 443\n",
			},
			want: "jp.lazerpenguin.com",
		},
		{
			name: "tolerates leading whitespace and trailing columns",
			profile: config.Profile{
				ConfigContent: "   remote \t tr.lazerpenguin.com 443 tcp-client\n",
			},
			want: "tr.lazerpenguin.com",
		},
		{
			name:    "ignores an unrelated endpoint",
			profile: config.Profile{Server: "vpn.example.com"},
		},
		{
			name: "ignores a lookalike domain",
			profile: config.Profile{
				ConfigContent: "remote evil-lazerpenguin.com 443\n",
			},
		},
		{
			name: "ignores a commented-out remote line",
			profile: config.Profile{
				ConfigContent: "#remote de.lazerpenguin.com 443\n",
			},
		},
		{
			// `remote-cert-tls` is the line that makes a naive `remote` prefix
			// match go wrong on every real OpenVPN config.
			name: "ignores remote-cert-tls",
			profile: config.Profile{
				ConfigContent: "client\nremote-cert-tls server\n",
			},
		},
		{
			name:    "ignores an empty profile",
			profile: config.Profile{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tunnelBearHostOf(tc.profile); got != tc.want {
				t.Fatalf("tunnelBearHostOf() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The relay's socket is kept out of the tunnel by `route <ip> 255.255.255.255
// net_gateway` lines, and net_gateway is an IPv4 concept. An IPv6 address in
// the pool would produce a relay whose own socket gets captured by the tunnel
// it is building, so resolution must stay IPv4-only.
func TestResolveTunnelBearHostStaysIPv4(t *testing.T) {
	ips, err := resolveTunnelBearHost("localhost")
	if err != nil {
		t.Skipf("localhost does not resolve in this environment: %v", err)
	}
	if len(ips) == 0 {
		t.Fatal("localhost resolved to nothing")
	}

	seen := map[string]bool{}
	for _, ip := range ips {
		if strings.Contains(ip, ":") {
			t.Fatalf("resolved to the non-IPv4 address %q", ip)
		}
		if seen[ip] {
			t.Fatalf("resolved to the duplicate address %q", ip)
		}
		seen[ip] = true
	}
}

func TestResolveTunnelBearHostRejectsAnEmptyName(t *testing.T) {
	if _, err := resolveTunnelBearHost(""); err == nil {
		t.Fatal("expected an error for an empty host, got none")
	}
}

// The connect-time view carries an ephemeral relay port and a plaintext
// password. The stored value must keep neither, because the caller writes the
// stored value back to disk after a successful connect.
func TestApplyTunnelBearConnectionLeavesTheStoredProfileAlone(t *testing.T) {
	published := discovery.BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)
	stored := config.Profile{
		ID:            "profile-1",
		Name:          "TunnelBear Germany",
		Protocol:      config.ProtocolOpenVPN,
		Category:      config.CategoryOpenVPN,
		ConfigContent: published,
		Server:        "de.lazerpenguin.com",
		Port:          discovery.TunnelBearDirectPort,
	}

	relayed := discovery.BuildTunnelBearConfig("de.lazerpenguin.com", 51820, []string{"203.0.113.7"})
	connect := applyTunnelBearConnection(stored, relayed, "user@example.com", "hunter2")

	if connect.ConfigContent != relayed {
		t.Fatal("the connect-time view did not receive the rewritten config")
	}
	if connect.Username != "user@example.com" || connect.Password != "hunter2" {
		t.Fatal("the connect-time view did not receive the account credentials")
	}

	if stored.ConfigContent != published {
		t.Fatal("the stored profile's config was rewritten in place")
	}
	if stored.Username != "" || stored.Password != "" {
		t.Fatal("the stored profile picked up the account credentials")
	}

	// The published form is the operator's own shape: the direct UDP endpoint,
	// no loopback relay, and no device name pinned.
	if strings.Contains(published, "127.0.0.1") {
		t.Fatal("the published form points at the loopback relay")
	}
	if !strings.Contains(published, "remote de.lazerpenguin.com 443") {
		t.Fatal("the published form does not dial the operator's direct endpoint")
	}
	if !strings.Contains(published, "proto udp") {
		t.Fatal("the published form is not the operator's UDP transport")
	}
	if strings.Contains(published, "dev tun0") {
		t.Fatal("the published form pins a device name the adapter will strip")
	}
	if !strings.Contains(published, "auth-user-pass") {
		t.Fatal("the published form must still ask for credentials at connect time")
	}

	// The rewritten form is the relay's shape.
	if !strings.Contains(relayed, "remote 127.0.0.1 51820 tcp-client") {
		t.Fatal("the rewritten config does not dial the relay")
	}
	if !strings.Contains(relayed, "route 203.0.113.7 255.255.255.255 net_gateway") {
		t.Fatal("the rewritten config would let the tunnel capture the relay's own socket")
	}
	if strings.Contains(relayed, "openvpn-server-ca.crt") {
		t.Fatal("the rewritten config references an external certificate file")
	}
	if strings.Contains(relayed, "explicit-exit-notify") {
		t.Fatal("the rewritten config carries a UDP-only directive on a TCP remote")
	}
}

// The published form is what the user sees in the profile list and what a
// fresh launch re-reads. It has to survive the store intact.
func TestPublishedTunnelBearProfileSurvivesTheStore(t *testing.T) {
	store := config.NewStoreWithDir(t.TempDir())

	published := discovery.BuildTunnelBearConfig("de.lazerpenguin.com", 0, nil)
	id, err := store.AddProfile(config.Profile{
		Name:          "TunnelBear Germany",
		Protocol:      config.ProtocolOpenVPN,
		Category:      config.CategoryOpenVPN,
		ConfigContent: published,
		Server:        "de.lazerpenguin.com",
		Port:          discovery.TunnelBearDirectPort,
	})
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	got, err := store.GetProfile(id)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.ConfigContent != published {
		t.Fatal("the stored config is no longer the published form")
	}
	if got.Username != "" || got.Password != "" {
		t.Fatal("credentials reached the profile store")
	}
	if got.Server != "de.lazerpenguin.com" {
		t.Fatalf("Server = %q, want the operator host", got.Server)
	}
}

// A user who downloaded one of the operator's `.ovpn` files by hand gets a
// profile the app cannot use as-is: those files carry `dev tun0`, which the
// adapter's sanitizer strips, and `ca openvpn-server-ca.crt`, which the adapter
// cannot resolve at all. The connect-time rewrite replaces the whole config, so
// such a profile must be recognised and must come out self-contained.
func TestManuallyImportedTunnelBearProfileIsRecognisedAndRepaired(t *testing.T) {
	// Shaped like the operator's published file.
	published := strings.Join([]string{
		"client",
		"dev tun0",
		"proto udp",
		"remote de.lazerpenguin.com 443",
		"resolv-retry infinite",
		"nobind",
		"persist-key",
		"persist-tun",
		"ca openvpn-server-ca.crt",
		"auth-user-pass",
		"cipher AES-256-CBC",
		"auth SHA256",
		"verb 3",
		"",
	}, "\n")

	profile := config.Profile{
		Name:          "tunnelbear-de",
		Protocol:      config.ProtocolOpenVPN,
		ConfigContent: published,
		// A hand-imported profile has no Server field filled in.
	}

	host := tunnelBearHostOf(profile)
	if host != "de.lazerpenguin.com" {
		t.Fatalf("a hand-imported TunnelBear profile was not recognised (host %q)", host)
	}

	rebuilt := discovery.BuildTunnelBearConfig(host, 51234, []string{"203.0.113.7"})
	if strings.Contains(rebuilt, "openvpn-server-ca.crt") {
		t.Fatal("the rebuilt config still references the external certificate file")
	}
	if !strings.Contains(rebuilt, "<ca>") || !strings.Contains(rebuilt, "</ca>") {
		t.Fatal("the rebuilt config does not inline the certificate")
	}
	if strings.Contains(rebuilt, "dev tun0") {
		t.Fatal("the rebuilt config still pins the device name the adapter strips")
	}
	if !strings.Contains(rebuilt, "dev tun\n") {
		t.Fatal("the rebuilt config does not ask for a plain tun device")
	}
}

// A profile that is not a TunnelBear endpoint must pass through untouched, so
// that an ordinary OpenVPN profile never has credentials injected into it.
func TestNonTunnelBearProfilesAreNotRewritten(t *testing.T) {
	ordinary := config.Profile{
		Name:          "My own server",
		Protocol:      config.ProtocolOpenVPN,
		ConfigContent: "client\ndev tun\nremote vpn.example.com 1194 udp\n",
		Server:        "vpn.example.com",
	}

	if host := tunnelBearHostOf(ordinary); host != "" {
		t.Fatalf("an ordinary OpenVPN profile was claimed by TunnelBear (host %q)", host)
	}
}
