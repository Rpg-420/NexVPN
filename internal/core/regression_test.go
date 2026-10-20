package core

import (
	"strings"
	"testing"
	"time"
)

// TestIsPrivate172 covers the RFC 1918 172.16.0.0/12 range helper that was
// previously duplicated (and divergently implemented) in app_network.go.
func TestIsPrivate172(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"172.16.0.1", true},
		{"172.19.0.1", true}, // the NexVPN-TUN range
		{"172.31.255.254", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"172.", false},
		{"172.abc.1.1", false},
		{"not-an-ip", false},
	}
	for _, c := range cases {
		if got := IsPrivate172(c.ip); got != c.want {
			t.Errorf("IsPrivate172(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

// TestRedactURLSecrets ensures subscription tokens embedded in query strings are
// never written to logs in clear text.
func TestRedactURLSecrets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // substring that must be present
		gone string // secret that must NOT appear
	}{
		{"token", "https://panel.example.com/sub?token=SECRET123&x=1", "***REDACTED***", "SECRET123"},
		{"plain key", "https://example.com/subscribe?key=deadbeef", "***REDACTED***", "deadbeef"},
		{"api_key", "https://a.com/s?api_key=XX", "***REDACTED***", "XX"},
		{"subid", "https://a.com/s?subid=42", "***REDACTED***", "42"},
		{"password", "https://a.com/s?password=P%40ss", "***REDACTED***", "P%40ss"},
	}
	for _, c := range cases {
		got := RedactURLSecrets(c.in)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: RedactURLSecrets(%q) = %q, expected it to contain %q", c.name, c.in, got, c.want)
		}
		if strings.Contains(got, c.gone) {
			t.Errorf("%s: secret %q leaked in %q", c.name, c.gone, got)
		}
	}

	// A URL without any sensitive parameter must be left intact.
	clean := "https://example.com/subscribe?theme=dark"
	if got := RedactURLSecrets(clean); got != clean {
		t.Errorf("clean URL was modified: %q -> %q", clean, got)
	}
}

// TestShouldAlertThrottles verifies the quota alert de-duplication window that
// gives lastAlerts its purpose.
func TestShouldAlertThrottles(t *testing.T) {
	qm := &QuotaMonitor{lastAlerts: make(map[string]time.Time)}
	const url = "https://example.com/sub?token=x"

	if !qm.shouldAlert(url) {
		t.Fatal("first alert for a URL must be allowed")
	}
	if qm.shouldAlert(url) {
		t.Fatal("a second alert inside the window must be throttled")
	}
	// A different URL is tracked independently.
	if !qm.shouldAlert("https://other.example.com/sub") {
		t.Fatal("alerts for distinct URLs must not interfere")
	}
	// Expiring the window re-enables the alert.
	qm.mu.Lock()
	qm.lastAlerts[url] = time.Now().Add(-alertWindow - time.Minute)
	qm.mu.Unlock()
	if !qm.shouldAlert(url) {
		t.Fatal("alert must be allowed again once the window elapsed")
	}
}

// TestRemoteServerIdleTimeout confirms the LAN remote service self-terminates
// after the configured inactivity window.
func TestRemoteServerIdleTimeout(t *testing.T) {
	rs := &RemoteServer{idleTimeout: DefaultIdleTimeout}
	if got := rs.IdleTimeout(); got != DefaultIdleTimeout {
		t.Fatalf("IdleTimeout() = %v, want %v", got, DefaultIdleTimeout)
	}

	rs.SetIdleTimeout(5 * time.Minute)
	if got := rs.IdleTimeout(); got != 5*time.Minute {
		t.Fatalf("after SetIdleTimeout, IdleTimeout() = %v, want 5m", got)
	}

	// A non-positive override must be ignored rather than disabling the guard.
	rs.SetIdleTimeout(0)
	if got := rs.IdleTimeout(); got != 5*time.Minute {
		t.Fatalf("SetIdleTimeout(0) changed the value to %v", got)
	}

	// touchActivity moves the deadline forward.
	rs.mu.Lock()
	rs.lastActivity = time.Now().Add(-time.Hour)
	rs.mu.Unlock()
	rs.touchActivity()
	rs.mu.RLock()
	recent := time.Since(rs.lastActivity) < time.Minute
	rs.mu.RUnlock()
	if !recent {
		t.Fatal("touchActivity did not refresh lastActivity")
	}
}

// TestDefaultIdleTimeoutIsSane guards against someone accidentally setting the
// LAN exposure window to something effectively permanent.
func TestDefaultIdleTimeoutIsSane(t *testing.T) {
	if DefaultIdleTimeout <= 0 || DefaultIdleTimeout > 2*time.Hour {
		t.Fatalf("DefaultIdleTimeout = %v, expected a bounded non-zero window", DefaultIdleTimeout)
	}
}
