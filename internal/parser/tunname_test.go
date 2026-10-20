package parser

import (
	"strings"
	"testing"

	"nexvpn/internal/config"
)

// TestTunIfaceNameHonoursTheEngineOverride pins the contract that lets the engine
// rename around a ghost Wintun adapter.
//
// A ghost is an adapter the Wintun driver still answers to but Windows cannot
// enumerate: `WintunCreateAdapter` reports the name as taken and
// `WintunOpenAdapter` returns a handle to nothing, so a core told to use that
// name exits during startup. The engine detects it and passes a different name
// through Settings.TunInterfaceName; if the config generator ignored that, the
// rename would silently do nothing and every connection would keep failing.
func TestTunIfaceNameHonoursTheEngineOverride(t *testing.T) {
	if got := tunIfaceName(config.Settings{}); got != config.DefaultTunInterfaceName {
		t.Errorf("empty settings must fall back to %q, got %q", config.DefaultTunInterfaceName, got)
	}

	const chosen = "NexVPN-TUN-3"
	if got := tunIfaceName(config.Settings{TunInterfaceName: chosen}); got != chosen {
		t.Errorf("the engine's override must win, got %q want %q", got, chosen)
	}

	// Whitespace is not a name.
	if got := tunIfaceName(config.Settings{TunInterfaceName: "   "}); got != config.DefaultTunInterfaceName {
		t.Errorf("a blank override must fall back to the default, got %q", got)
	}
}

// TestGeneratedTunInboundUsesTheOverriddenName checks the name actually reaches
// the emitted config, which is the only thing the core ever sees.
func TestGeneratedTunInboundUsesTheOverriddenName(t *testing.T) {
	outbound := map[string]interface{}{
		"type":        "vless",
		"tag":         "proxy",
		"server":      "example.com",
		"server_port": 443,
	}
	settings := config.Settings{
		ProxyMode:        "tun",
		TunInterfaceName: "NexVPN-TUN-4",
	}

	body, err := BuildSingBoxConfigAdvanced(outbound, "tun", "bypass_iran", "example.com", settings)
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced: %v", err)
	}
	if !strings.Contains(body, `"NexVPN-TUN-4"`) {
		t.Errorf("the generated config must carry the overridden adapter name")
	}
	if strings.Contains(body, `"interface_name": "NexVPN-TUN"`) {
		t.Errorf("the generated config must not also carry the default name")
	}
}
