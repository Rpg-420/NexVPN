package parser

import "testing"

// TestIsXhttpURI guards the transport-detection helper that was left mangled
// (unbalanced block + dead prologue) by an earlier edit and never compiled.
func TestIsXhttpURI(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want bool
	}{
		{"xhttp", "vless://uuid@example.com:443?type=xhttp&path=%2F", true},
		{"splithttp", "vless://uuid@example.com:443?type=splithttp", true},
		{"uppercase", "vless://uuid@example.com:443?type=XHTTP", true},
		{"with fragment", "vless://uuid@example.com:443?type=xhttp#My-Node", true},
		{"ws is not xhttp", "vless://uuid@example.com:443?type=ws&path=%2F", false},
		{"grpc is not xhttp", "vless://uuid@example.com:443?type=grpc", false},
		{"no type", "vless://uuid@example.com:443", false},
		{"empty", "", false},
		{"garbage fallback", "not-a-url?type=xhttp", true},
	}
	for _, c := range cases {
		if got := IsXhttpURI(c.uri); got != c.want {
			t.Errorf("%s: IsXhttpURI(%q) = %v, want %v", c.name, c.uri, got, c.want)
		}
	}
}

// TestParseWireguardOutbound verifies the WireGuard URI parser still returns a
// usable outbound after the duplicated-block corruption in
// convertWireGuardOutboundToEndpoint was repaired.
func TestParseWireguardOutbound(t *testing.T) {
	uri := "wireguard://privatekey123@162.159.192.1:2408?public_key=peerkey456&ip=172.16.0.2%2F32"
	ob, err := parseWireguardOutbound(uri)
	if err != nil {
		t.Fatalf("parseWireguardOutbound returned error: %v", err)
	}
	if ob == nil {
		t.Fatal("parseWireguardOutbound returned nil outbound")
	}
	if ob["type"] != "wireguard" {
		t.Errorf("type = %v, want wireguard", ob["type"])
	}
	if ob["server"] == "" || ob["server"] == nil {
		t.Errorf("server was not parsed: %v", ob["server"])
	}
}

// TestConvertWireGuardOutboundToEndpoint covers the endpoint converter that had
// three duplicated statements and two unclosed blocks.
func TestConvertWireGuardOutboundToEndpoint(t *testing.T) {
	ob := map[string]interface{}{
		"type":            "wireguard",
		"tag":             "proxy",
		"address":         []string{"172.16.0.2/32"},
		"private_key":     "priv",
		"peer_public_key": "peer",
		"server":          "162.159.192.1",
		"server_port":     2408,
		"reserved":        []int{1, 2, 3},
		"mtu":             1280,
		"detour":          "warp-hop1",
	}

	ep := convertWireGuardOutboundToEndpoint(ob)
	if ep["type"] != "wireguard" {
		t.Errorf("type = %v, want wireguard", ep["type"])
	}
	if ep["tag"] != "proxy" {
		t.Errorf("tag = %v, want proxy", ep["tag"])
	}
	if ep["private_key"] != "priv" {
		t.Errorf("private_key = %v, want priv", ep["private_key"])
	}
	if ep["mtu"] != 1280 {
		t.Errorf("mtu = %v, want 1280", ep["mtu"])
	}
	if ep["detour"] != "warp-hop1" {
		t.Errorf("detour = %v, want warp-hop1", ep["detour"])
	}
	peers, ok := ep["peers"].([]map[string]interface{})
	if !ok || len(peers) != 1 {
		t.Fatalf("peers malformed: %#v", ep["peers"])
	}
	if peers[0]["port"] != 2408 {
		t.Errorf("peer port = %v, want 2408", peers[0]["port"])
	}
	res, ok := peers[0]["reserved"].([]int)
	if !ok || len(res) != 3 {
		t.Errorf("reserved = %#v, want [1 2 3]", peers[0]["reserved"])
	}
}

// TestConvertWireGuardOutboundFallsBackToDefaultAddress ensures a profile with
// no address still produces the standard WARP client addresses instead of an
// empty endpoint.
func TestConvertWireGuardOutboundFallsBackToDefaultAddress(t *testing.T) {
	ep := convertWireGuardOutboundToEndpoint(map[string]interface{}{
		"type":        "wireguard",
		"private_key": "priv",
	})
	addrs, ok := ep["address"].([]string)
	if !ok || len(addrs) == 0 {
		t.Fatalf("expected fallback addresses, got %#v", ep["address"])
	}
}
