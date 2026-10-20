package dnsscan

import (
	"testing"
	"time"
)

func TestPredefinedServersList(t *testing.T) {
	if len(PredefinedServers) == 0 {
		t.Fatal("PredefinedServers list should not be empty")
	}

	hasDoH := false
	for _, s := range PredefinedServers {
		if s.Name == "" || s.Address == "" {
			t.Errorf("invalid server configuration: %+v", s)
		}
		if s.IsDoH {
			hasDoH = true
			if s.DoHURL == "" {
				t.Errorf("DoH server %s must have a valid DoHURL", s.Name)
			}
		}
	}

	if !hasDoH {
		t.Error("at least one predefined server must support DoH")
	}
}

func TestTestDoHDNSStructure(t *testing.T) {
	_, err := testDoHDNS("http://127.0.0.1:59999/dns-query", "cloudflare.com", 100*time.Millisecond)
	if err == nil {
		t.Error("expected error for unreachable DoH endpoint, got nil")
	}
}
