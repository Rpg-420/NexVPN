package parser

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// proxyPortOf extracts the port from an httptest server URL.
func proxyPortOf(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	_, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split %q: %v", u.Host, err)
	}
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}
	return port
}

// A proxy whose upstream is dead still answers — sing-box replies 502 from its
// inbound when the outbound dial fails, and Go's http.Client returns a nil
// error for any HTTP status. Regression guard: the connect-time tunnel
// verification used to accept that as "reachable", so the app announced
// "connected" for a tunnel that was provably dead.
func TestRealDelayPingRejectsHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An HTTP proxy receives the request in absolute form.
		if r.URL.Host == "" {
			t.Errorf("expected an absolute-form request line through the proxy, got %q", r.URL.String())
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := RealDelayPing(proxyPortOf(t, srv.URL), "http://example.com/generate_204", 3*time.Second)
	if err == nil {
		t.Fatal("RealDelayPing accepted a 502 from the proxy; a dead tunnel would be reported as healthy")
	}
}

// The happy path must still succeed, so the guard above cannot be satisfied by
// simply failing everything.
func TestRealDelayPingAccepts204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	latency, err := RealDelayPing(proxyPortOf(t, srv.URL), "http://example.com/generate_204", 3*time.Second)
	if err != nil {
		t.Fatalf("RealDelayPing rejected a 204: %v", err)
	}
	if latency < 0 {
		t.Fatalf("unexpected latency %d", latency)
	}
}
