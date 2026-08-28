package core

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRemoteServerAuthAndHeaderOnlyAuthorization(t *testing.T) {
	rs := &RemoteServer{
		authToken:   "token-123",
		authPin:     "123456",
		authResetAt: time.Now().Add(time.Minute),
	}

	queryReq := httptest.NewRequest("GET", "/api/status?token=token-123", nil)
	if rs.isAuthorized(queryReq) {
		t.Fatal("query-string token must not authorize a request")
	}

	headerReq := httptest.NewRequest("GET", "/api/status", nil)
	headerReq.Header.Set("X-NexVPN-Token", "token-123")
	if !rs.isAuthorized(headerReq) {
		t.Fatal("valid header token must authorize a request")
	}

	badPin := httptest.NewRequest("POST", "/api/auth", strings.NewReader(`{"pin":"000000"}`))
	badPin.Header.Set("Content-Type", "application/json")
	badResp := httptest.NewRecorder()
	rs.handleAuth(badResp, badPin)
	if badResp.Code != 401 {
		t.Fatalf("bad PIN status = %d, want 401", badResp.Code)
	}

	goodPin := httptest.NewRequest("POST", "/api/auth", strings.NewReader(`{"pin":"123456"}`))
	goodPin.Header.Set("Content-Type", "application/json")
	goodResp := httptest.NewRecorder()
	rs.handleAuth(goodResp, goodPin)
	if goodResp.Code != 200 || !strings.Contains(goodResp.Body.String(), `"success":true`) {
		t.Fatalf("valid PIN response = %d %s", goodResp.Code, goodResp.Body.String())
	}
}

func TestRemoteServerAuthAttemptLimit(t *testing.T) {
	rs := &RemoteServer{authPin: "123456", authToken: "token", authResetAt: time.Now().Add(time.Minute)}
	for i := 0; i < 5; i++ {
		if !rs.allowAuthAttempt() {
			t.Fatalf("attempt %d unexpectedly rejected", i+1)
		}
	}
	if rs.allowAuthAttempt() {
		t.Fatal("sixth authentication attempt must be rejected")
	}
}

func TestRemoteServerURLLifecycle(t *testing.T) {
	rs := &RemoteServer{port: 12345}
	if got := rs.GetURL(); got != "" {
		t.Fatalf("stopped server URL = %q, want empty", got)
	}
	rs.running = true
	if got := rs.GetURL(); got != "http://127.0.0.1:12345" {
		t.Fatalf("running server URL = %q", got)
	}
	if strings.Contains(rs.GetURLWithToken(), "token=") {
		t.Fatal("GetURLWithToken must not expose bearer token in URL")
	}
}
