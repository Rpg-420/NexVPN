package core

import (
	"encoding/json"
	"testing"

	"nexvpn/internal/config"
)

func TestSingBoxConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()
	settings := config.Settings{
		RoutingMode: "bypass_iran",
		ProxyMode:   "tun",
		IPStrategy:  "ipv4_only",
	}

	mockOutbound := map[string]interface{}{
		"type":        "vless",
		"tag":         "proxy",
		"server":      "example.com",
		"server_port": 443,
		"uuid":        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}

	jsonStr, err := builder.
		WithStandardInbounds(settings.ProxyMode, settings.GameModeEnabled).
		WithProxyOutbound(mockOutbound).
		WithSmartDNS("1.1.1.1", "8.8.8.8", settings.IPStrategy).
		WithRoutingRules(settings).
		Build()

	if err != nil {
		t.Fatalf("Builder.Build() failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}

	if _, ok := parsed["inbounds"]; !ok {
		t.Errorf("Expected 'inbounds' field in config")
	}
	if _, ok := parsed["outbounds"]; !ok {
		t.Errorf("Expected 'outbounds' field in config")
	}
	if _, ok := parsed["dns"]; !ok {
		t.Errorf("Expected 'dns' field in config")
	}
	if _, ok := parsed["route"]; !ok {
		t.Errorf("Expected 'route' field in config")
	}
}
