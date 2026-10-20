package protocols

import "testing"

// tunInterfaceName is the switch that decides whether Start() waits for a
// Wintun adapter (TUN config) or keeps the flat 1.5s liveness window
// (proxy-only config). Getting it wrong silently reintroduces the bug where the
// core passed the startup probe, the engine announced "connected", and the core
// died ~13s later on the TUN inbound.
func TestTunInterfaceName(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		want string
	}{
		{
			name: "tun inbound is detected",
			cfg:  `{"inbounds":[{"type":"tun","interface_name":"NexVPN-TUN"},{"type":"mixed","listen_port":10808}],"outbounds":[]}`,
			want: "NexVPN-TUN",
		},
		{
			name: "proxy-only config has no tun",
			cfg:  `{"inbounds":[{"type":"mixed","listen_port":10808},{"type":"http","listen_port":10809}],"outbounds":[]}`,
			want: "",
		},
		{
			name: "tun without an explicit interface_name is ignored",
			cfg:  `{"inbounds":[{"type":"tun","address":["172.19.0.1/30"]}],"outbounds":[]}`,
			want: "",
		},
		{
			name: "mixed inbounds: first tun wins",
			cfg:  `{"inbounds":[{"type":"socks"},{"type":"tun","interface_name":"NexVPN-OVPN"}],"outbounds":[]}`,
			want: "NexVPN-OVPN",
		},
		{
			name: "malformed json degrades to the fast path",
			cfg:  `{"inbounds":[`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tunInterfaceName(tc.cfg); got != tc.want {
				t.Fatalf("tunInterfaceName() = %q, want %q", got, tc.want)
			}
		})
	}
}
