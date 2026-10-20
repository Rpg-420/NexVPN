package config

import "testing"

// TestEffectiveCategory covers the shared category resolver that PingAllProfiles
// now uses. Legacy profiles with an empty Category must be classified from their
// protocol instead of being silently skipped by category filters.
func TestEffectiveCategory(t *testing.T) {
	cases := []struct {
		name     string
		profile  Profile
		expected ProfileCategory
	}{
		{"explicit category wins", Profile{Category: CategoryOpenVPN, Protocol: ProtocolXray}, CategoryOpenVPN},
		{"wireguard infers warp", Profile{Protocol: ProtocolWireGuard}, CategoryWireGuardWarp},
		{"openvpn infers openvpn", Profile{Protocol: ProtocolOpenVPN}, CategoryOpenVPN},
		{"singbox infers v2ray", Profile{Protocol: ProtocolSingBox}, CategoryV2Ray},
		{"xray infers v2ray", Profile{Protocol: ProtocolXray}, CategoryV2Ray},
		{"unknown protocol falls back to v2ray", Profile{Protocol: Protocol("mystery")}, CategoryV2Ray},
	}
	for _, c := range cases {
		if got := c.profile.EffectiveCategory(); got != c.expected {
			t.Errorf("%s: EffectiveCategory() = %q, want %q", c.name, got, c.expected)
		}
	}
}

// TestGetSubscriptionURLs verifies the deduplicating accessor used by the
// background quota monitor.
func TestGetSubscriptionURLs(t *testing.T) {
	s := &Store{
		data: StoreData{
			Profiles: []Profile{
				{ID: "1", SubscriptionURL: "https://a.example.com/sub?token=1"},
				{ID: "2", SubscriptionURL: "https://a.example.com/sub?token=1"}, // duplicate
				{ID: "3", SubscriptionURL: "  https://b.example.com/sub  "},     // needs trimming
				{ID: "4", SubscriptionURL: ""},                                  // ignored
				{ID: "5"},                                                       // ignored
			},
		},
	}

	got := s.GetSubscriptionURLs()
	if len(got) != 2 {
		t.Fatalf("GetSubscriptionURLs() returned %d URLs (%v), want 2", len(got), got)
	}
	seen := map[string]bool{}
	for _, u := range got {
		seen[u] = true
	}
	if !seen["https://a.example.com/sub?token=1"] {
		t.Errorf("missing first subscription URL: %v", got)
	}
	if !seen["https://b.example.com/sub"] {
		t.Errorf("trimmed URL missing: %v", got)
	}
}

// TestGetSubscriptionURLsEmpty makes sure an empty store does not panic or
// return a nil-backed slice that callers might mis-handle.
func TestGetSubscriptionURLsEmpty(t *testing.T) {
	s := &Store{}
	if got := s.GetSubscriptionURLs(); len(got) != 0 {
		t.Fatalf("expected no URLs, got %v", got)
	}
}
