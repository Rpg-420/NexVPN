package core

import (
	"net/http"
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
		if !rs.authAllowed() {
			t.Fatalf("attempt %d unexpectedly rejected", i+1)
		}
		rs.recordAuthFailure()
	}
	if rs.authAllowed() {
		t.Fatal("sixth authentication attempt must be rejected")
	}
}

// A correct PIN must not consume the attempt budget.
//
// Attempts used to be counted before the comparison, so five successful sign-ins
// inside the window locked the user out of their own controller and reported
// "Invalid PIN code" for a PIN that was right.
func TestRemoteServerSuccessfulAuthDoesNotConsumeAttempts(t *testing.T) {
	rs := &RemoteServer{authPin: "123456", authToken: "token", authResetAt: time.Now().Add(time.Minute)}

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/auth", strings.NewReader(`{"pin":"123456"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		rs.handleAuth(w, req)
		if w.Code != 200 {
			t.Fatalf("successful sign-in %d status = %d, want 200", i+1, w.Code)
		}
	}

	// The budget is untouched, so a wrong PIN is still answered as "invalid"
	// rather than as "too many attempts".
	bad := httptest.NewRequest("POST", "/api/auth", strings.NewReader(`{"pin":"000000"}`))
	bad.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	rs.handleAuth(badW, bad)
	if badW.Code != http.StatusUnauthorized {
		t.Fatalf("wrong PIN after successful sign-ins status = %d, want 401", badW.Code)
	}
}

// An exhausted window must be answered with 429, and must not keep comparing
// PINs — otherwise the rate limit only changes the error message.
func TestRemoteServerAuthLimitReturnsTooManyRequests(t *testing.T) {
	rs := &RemoteServer{
		authPin:      "123456",
		authToken:    "token",
		authResetAt:  time.Now().Add(time.Minute),
		authAttempts: maxAuthAttempts,
	}

	req := httptest.NewRequest("POST", "/api/auth", strings.NewReader(`{"pin":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	rs.handleAuth(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("exhausted window status = %d, want 429", w.Code)
	}
}

func TestRemoteServerURLLifecycle(t *testing.T) {
	rs := &RemoteServer{port: 12345}
	if got := rs.GetURL(); got != "" {
		t.Fatalf("stopped server URL = %q, want empty", got)
	}
	rs.running = true
	got := rs.GetURL()
	if !strings.HasPrefix(got, "http://") || !strings.HasSuffix(got, ":12345") {
		t.Fatalf("running server URL = %q, want http://<ip>:12345", got)
	}
	if strings.Contains(rs.GetURLWithToken(), "token=") {
		t.Fatal("GetURLWithToken must not expose bearer token in URL")
	}
}
