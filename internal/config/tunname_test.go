package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEffectiveTunInterfaceName pins the fallback rules for the Wintun adapter
// name.
//
// The field is runtime-only (json:"-") and is set by the engine when it has to
// avoid a ghost adapter — one the Wintun driver still answers to but Windows
// cannot enumerate. Every consumer (the config generator, the MTU tuner, the
// cleanup script) has to agree on the same name, so the default has to be stable
// and an empty override must never leak through as an empty name.
func TestEffectiveTunInterfaceName(t *testing.T) {
	if DefaultTunInterfaceName == "" {
		t.Fatal("the default adapter name must not be empty")
	}

	cases := []struct {
		name     string
		override string
		want     string
	}{
		{"no override", "", DefaultTunInterfaceName},
		{"blank override", "   ", DefaultTunInterfaceName},
		{"engine override", "NexVPN-TUN-2", "NexVPN-TUN-2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Settings{TunInterfaceName: tc.override}
			if got := s.EffectiveTunInterfaceName(); got != tc.want {
				t.Errorf("EffectiveTunInterfaceName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTunInterfaceNameIsNotPersisted guards the reason the field is tagged
// `json:"-"`: it is decided per connection, so writing it into config.json would
// freeze one connection's choice and hand it to the next launch — which is
// exactly the state that made a ghost adapter permanent.
func TestTunInterfaceNameIsNotPersisted(t *testing.T) {
	s := Settings{TunInterfaceName: "NexVPN-TUN-7"}
	blob, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(blob)
	if strings.Contains(got, "NexVPN-TUN-7") || strings.Contains(got, "TunInterfaceName") {
		t.Errorf("the runtime adapter name must not be serialised, got: %s", got)
	}
}
