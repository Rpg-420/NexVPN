package parser

import (
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMeasureLatencyAcceptsAPlainListener is the regression the server list was
// complaining about: an endpoint that answers TCP but never completes a TLS
// handshake — every `ip:443` Reality node, and any OpenVPN relay — used to be
// reported as dead because the latency number came from TCPPing.
func TestMeasureLatencyAcceptsAPlainListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Accept until the listener is closed at the end of the test: `Accept`
		// errors out once `ln.Close()` runs, which is what ends this loop. A fixed
		// iteration count would leave the goroutine parked in Accept forever.
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Accept and hold briefly, then drop: a real server's behaviour, and
			// nothing that ever speaks TLS.
			go func(c net.Conn) { <-time.After(20 * time.Millisecond); c.Close() }(conn)
		}
	}()

	host, port := splitAddr(t, ln.Addr().String())
	latency := MeasureLatency(host, port, 2*time.Second)
	if latency <= 0 {
		t.Errorf("MeasureLatency = %d, want a positive number for an endpoint that accepts TCP", latency)
	}
	ln.Close()
	<-done
}

func TestMeasureLatencyRefusesWhatIsNotThere(t *testing.T) {
	if got := MeasureLatency("", 443, time.Second); got != -1 {
		t.Errorf("MeasureLatency with no server = %d, want -1", got)
	}

	// Bind then close a port so the number is (almost certainly) refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	host, port := splitAddr(t, addr)
	if got := MeasureLatency(host, port, 500*time.Millisecond); got != -1 {
		t.Errorf("MeasureLatency against a closed port = %d, want -1", got)
	}
}

// A Shadowsocks link whose cipher/password half cannot be decoded used to be
// written into the config with both fields empty. Both cores then refused to
// start, with messages that named neither the profile nor the missing piece:
// sing-box said `initialize outbound[2]: unknown method:` and xray said
// `Shadowsocks password is not specified`.
func TestParseShadowsocksRejectsAnIncompleteLink(t *testing.T) {
	broken := []string{
		"ss://203.0.113.9:8388", // no userinfo at all
		"ss://" + base64.StdEncoding.EncodeToString([]byte("203.0.113.9:8388")), // base64 host, no credentials
		"ss://:pass@203.0.113.9:8388",                                           // empty cipher
	}

	for _, uri := range broken {
		if _, err := ParseOutboundFromURI(uri); err == nil {
			t.Errorf("ParseOutboundFromURI(%q) accepted an incomplete link; the cores will fail with a message that blames nothing", uri)
		} else if !strings.Contains(strings.ToLower(err.Error()), "cipher") &&
			!strings.Contains(strings.ToLower(err.Error()), "password") {
			t.Errorf("ParseOutboundFromURI(%q) error is not actionable: %v", uri, err)
		}
	}

	valid := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:s3cret")) + "@203.0.113.9:8388#Teheran"
	outbound, err := ParseOutboundFromURI(valid)
	if err != nil {
		t.Fatalf("a complete link was rejected: %v", err)
	}
	if outbound["method"] != "aes-256-gcm" || outbound["password"] != "s3cret" {
		t.Errorf("credentials parsed wrong: method=%v password=%v", outbound["method"], outbound["password"])
	}
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("port %q is not numeric: %v", portStr, err)
	}
	return host, port
}
