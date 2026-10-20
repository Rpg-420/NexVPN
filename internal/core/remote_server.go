package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
)

// DefaultIdleTimeout is how long the LAN remote-control service stays alive
// without any authenticated request before it shuts itself down. This limits
// the window in which the PIN entry page is reachable from other devices.
const DefaultIdleTimeout = 30 * time.Minute

// maxAuthAttempts is the number of failed PIN attempts tolerated inside
// authWindow before the endpoint starts rejecting everything.
const (
	maxAuthAttempts = 5
	authWindow      = time.Minute
)

type RemoteServer struct {
	mu           sync.RWMutex
	httpServer   *http.Server
	listener     net.Listener
	port         int
	running      bool
	authToken    string
	authPin      string
	authAttempts int
	authResetAt  time.Time
	lastActivity time.Time
	idleTimeout  time.Duration
	// requireLAN restricts the listener to loopback + RFC1918 peers. It is on
	// by default: the endpoint is meant for phones on the same Wi-Fi, and
	// there is no reason for a public IP or a neighbouring guest network to
	// even reach the PIN prompt.
	requireLAN bool
	// useTLS wraps the listener in TLS with an in-memory self-signed
	// certificate, so the 6-digit PIN is never sent in the clear.
	useTLS  bool
	tlsCert *tls.Certificate
	lanIP   string
	// scheme is "https" when TLS is active, otherwise "http". It is what
	// GetURL advertises to the paired phone.
	scheme string
	// stopCh is closed when the server stops, so the idle watchdog of a given
	// run exits instead of lingering (and so a restart cannot accumulate them).
	stopCh chan struct{}
	engine *Engine
	store  *config.Store
}

var (
	globalRemoteServer     *RemoteServer
	globalRemoteServerOnce sync.Once
)

func GetRemoteServer(engine *Engine, store *config.Store) *RemoteServer {
	globalRemoteServerOnce.Do(func() {
		globalRemoteServer = &RemoteServer{
			port:        10815,
			engine:      engine,
			store:       store,
			authResetAt: time.Now().Add(time.Minute),
			idleTimeout: DefaultIdleTimeout,
			requireLAN:  true,
			useTLS:      true,
		}
		globalRemoteServer.generateCredentials()
	})
	return globalRemoteServer
}

func (rs *RemoteServer) generateCredentials() {
	// Generate 32-byte secure random token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		rs.authToken = ""
		rs.authPin = ""
		return
	}
	rs.authToken = hex.EncodeToString(tokenBytes)

	// Generate 6-digit random PIN
	pinNum, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		rs.authToken = ""
		rs.authPin = ""
		return
	}
	rs.authPin = fmt.Sprintf("%06d", pinNum.Int64()+100000)
}

func (rs *RemoteServer) GetAuthPIN() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.authPin
}

// generateSelfSignedCert builds an ephemeral ECDSA P-256 certificate valid for
// the loopback and LAN IPs the phone will connect to.
//
// The certificate lives only in memory for the lifetime of the process: there
// is nothing to persist, nothing to leak, and nothing to rotate.
func generateSelfSignedCert(lanIP string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serialMax := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialMax)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	if lanIP != "" {
		if parsed := net.ParseIP(lanIP); parsed != nil {
			ips = append(ips, parsed)
		}
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "NexVPN Remote", Organization: []string{"NexVPN"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
		DNSNames:              []string{"localhost", "nexvpn.local"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
	return cert, nil
}

// isLANPeer reports whether the connection originates from a device we are
// willing to serve: loopback, or an RFC1918 / link-local / unique-local peer.
//
// SECURITY: without this check the server is reachable from any routable
// address that can hit the port - a public IP if the machine is directly
// exposed, or a neighbouring guest Wi-Fi segment.
func isLANPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	// IsPrivate() only covers 10/8, 172.16/12 and 192.168/16. CGNAT
	// (100.64/10) shows up on some ISP routers and is equally non-routable.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

func (rs *RemoteServer) GetAuthToken() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.authToken
}

func (rs *RemoteServer) Start(port int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.running {
		return nil
	}
	if port <= 0 {
		port = 10815
	}
	rs.port = port
	rs.generateCredentials()
	if rs.authToken == "" || rs.authPin == "" {
		return fmt.Errorf("failed to generate remote credentials")
	}
	if rs.idleTimeout <= 0 {
		rs.idleTimeout = DefaultIdleTimeout
	}
	rs.lastActivity = time.Now()

	mux := http.NewServeMux()

	// Authentication Endpoint
	mux.HandleFunc("/api/auth", rs.handleAuth)

	// Protected API Endpoints
	mux.HandleFunc("/api/status", rs.handleStatus)
	mux.HandleFunc("/api/connect", rs.handleConnect)
	mux.HandleFunc("/api/disconnect", rs.handleDisconnect)

	// Dynamic Subscription Endpoints (for mobile iOS/Android clients)
	mux.HandleFunc("/sub", rs.handleSubscription)
	mux.HandleFunc("/sub/all", rs.handleSubscription)
	mux.HandleFunc("/api/subscription", rs.handleSubscription)

	// Responsive Web Mobile UI
	mux.HandleFunc("/", rs.handleWebUI)

	// Bind on 0.0.0.0 so devices on local Wi-Fi / LAN network can connect.
	// Every accepted connection is filtered by isLANPeer before it can reach
	// the mux, so "0.0.0.0" here means "the LAN interface", not "the internet".
	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)
	rawListener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("remote server listen failed: %w", err)
	}

	// Wrap the listener so non-LAN peers are dropped before any HTTP parsing.
	var listener net.Listener = rawListener
	if rs.requireLAN {
		listener = &lanGuardListener{Listener: rawListener}
	}

	scheme := "http"
	if rs.useTLS {
		lanIP := getLocalLANIP()
		cert, certErr := generateSelfSignedCert(lanIP)
		if certErr != nil {
			// Fall back to plain HTTP rather than refusing to start, but make
			// the downgrade visible in the log.
			rs.log("warn", fmt.Sprintf("Remote control TLS unavailable (%v); continuing over plain HTTP on the LAN", certErr))
			rs.useTLS = false
			rs.tlsCert = nil
		} else {
			rs.tlsCert = cert
			rs.lanIP = lanIP
			listener = tls.NewListener(listener, &tls.Config{
				Certificates: []tls.Certificate{*cert},
				MinVersion:   tls.VersionTLS12,
			})
			scheme = "https"
		}
	}
	rs.scheme = scheme

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           securityHeadersMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// The remote UI is same-origin only; no third-party embedding.
		ErrorLog: nil,
	}

	rs.httpServer = server
	rs.listener = listener
	rs.running = true

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			rs.mu.Lock()
			rs.running = false
			rs.mu.Unlock()
		}
	}()

	// Auto-stop the service after a period of inactivity so the PIN entry page
	// is not left reachable from the whole LAN indefinitely.
	stopCh := make(chan struct{})
	rs.stopCh = stopCh
	go rs.idleWatchdog(stopCh)

	return nil
}

// touchActivity records that an authenticated request was served. It is called
// on every authorized API call so the idle watchdog only fires when the service
// is genuinely unused.
func (rs *RemoteServer) touchActivity() {
	rs.mu.Lock()
	rs.lastActivity = time.Now()
	rs.mu.Unlock()
}

// idleWatchdog stops the remote server once no authenticated request has been
// seen for idleTimeout. It exits as soon as the server is stopped, or when its
// own run is superseded (stopCh closed).
func (rs *RemoteServer) idleWatchdog(stopCh chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
		}

		rs.mu.RLock()
		last := rs.lastActivity
		timeout := rs.idleTimeout
		rs.mu.RUnlock()

		if timeout <= 0 {
			timeout = DefaultIdleTimeout
		}
		if time.Since(last) >= timeout {
			rs.log("warn", fmt.Sprintf("Remote control service stopped automatically after %v of inactivity", timeout.Round(time.Minute)))
			rs.Stop()
			return
		}
	}
}

// log writes to the engine log when available, otherwise silently discards.
func (rs *RemoteServer) log(level, msg string) {
	if rs.engine != nil {
		rs.engine.AddLog(level, msg, "remote")
	}
}

// SetIdleTimeout overrides the inactivity window (used by settings/tests).
func (rs *RemoteServer) SetIdleTimeout(d time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if d > 0 {
		rs.idleTimeout = d
	}
}

// IdleTimeout returns the configured inactivity window.
func (rs *RemoteServer) IdleTimeout() time.Duration {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	if rs.idleTimeout <= 0 {
		return DefaultIdleTimeout
	}
	return rs.idleTimeout
}

func (rs *RemoteServer) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = rs.httpServer.Shutdown(ctx)
		rs.httpServer = nil
		rs.listener = nil
	}
	// Signal this run's idle watchdog to exit.
	if rs.stopCh != nil {
		select {
		case <-rs.stopCh:
		default:
			close(rs.stopCh)
		}
		rs.stopCh = nil
	}
	rs.running = false
	rs.scheme = ""
	rs.listener = nil
	// Any credentials minted for this run are dead; the next Start() mints
	// fresh ones so a leaked token cannot be replayed after a restart.
	rs.authToken = ""
	rs.authPin = ""
}

// IsRunning reports whether the LAN controller is currently serving.
func (rs *RemoteServer) IsRunning() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.running
}

// TransportSecurity describes how the LAN controller is protected, for display
// in the desktop UI so the user can see exactly what they enabled.
type TransportSecurity struct {
	TLSActive       bool   `json:"tls_active"`
	LANOnly         bool   `json:"lan_only"`
	Scheme          string `json:"scheme"`
	PINLength       int    `json:"pin_length"`
	MaxFailedPins   int    `json:"max_failed_pins"`
	IdleTimeoutMins int    `json:"idle_timeout_minutes"`
}

// GetTransportSecurity returns the live security posture of the controller.
func (rs *RemoteServer) GetTransportSecurity() TransportSecurity {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	scheme := rs.scheme
	if scheme == "" {
		scheme = "http"
	}
	return TransportSecurity{
		TLSActive:       rs.tlsCert != nil && scheme == "https",
		LANOnly:         rs.requireLAN,
		Scheme:          scheme,
		PINLength:       6,
		MaxFailedPins:   maxAuthAttempts,
		IdleTimeoutMins: int(rs.idleTimeout.Minutes()),
	}
}

// SetSecureDefaults configures the LAN controller policy. It must be called
// before Start; it exists so a future settings toggle can disable the LAN
// guard or TLS, and so tests can exercise both paths.
func (rs *RemoteServer) SetSecureDefaults(lanOnly, tls bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.running {
		return
	}
	rs.requireLAN = lanOnly
	rs.useTLS = tls
}

func (rs *RemoteServer) GetURL() string {
	rs.mu.RLock()
	port := rs.port
	running := rs.running
	scheme := rs.scheme
	rs.mu.RUnlock()
	if !running {
		return ""
	}
	if scheme == "" {
		scheme = "http"
	}
	ip := getLocalLANIP()
	return fmt.Sprintf("%s://%s:%d", scheme, ip, port)
}

func (rs *RemoteServer) GetURLWithToken() string {
	// Do not put bearer credentials in URLs. Callers must use Authorization or
	// X-NexVPN-Token headers after an explicit pairing step.
	return rs.GetURL()
}

// IsPrivate172 reports whether ipStr falls inside the 172.16.0.0/12 private
// range. Exported so other packages (e.g. the LAN bridge in app_network.go) use
// this single implementation instead of keeping a divergent copy.
func IsPrivate172(ipStr string) bool {
	if !strings.HasPrefix(ipStr, "172.") {
		return false
	}
	parts := strings.Split(ipStr, ".")
	if len(parts) < 2 {
		return false
	}
	second := 0
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
		second = second*10 + int(c-'0')
	}
	return second >= 16 && second <= 31
}

func getLocalLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipStr := ipNet.IP.String()
				if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || IsPrivate172(ipStr) {
					return ipStr
				}
			}
		}
	}
	return "127.0.0.1"
}

// lanGuardListener rejects connections from addresses that are not on the
// local network before they ever reach the HTTP handler or the TLS handshake.
//
// This is a defence-in-depth companion to the PIN: even if the PIN were weak,
// a remote host simply cannot complete a connection.
type lanGuardListener struct {
	net.Listener
}

func (l *lanGuardListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if isLANPeer(conn.RemoteAddr().String()) {
			return conn, nil
		}
		// Silently drop: no HTTP error, no TLS alert, no information leak.
		_ = conn.Close()
	}
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// authAllowed reports whether another PIN attempt may be made in the current
// window. It deliberately does not consume an attempt — the caller records a
// failure only once the PIN has actually been compared and rejected.
func (rs *RemoteServer) authAllowed() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if time.Now().After(rs.authResetAt) {
		rs.authAttempts = 0
		rs.authResetAt = time.Now().Add(authWindow)
	}
	return rs.authAttempts < maxAuthAttempts
}

// recordAuthFailure consumes one attempt in the current window.
//
// Attempts used to be counted up front, before the comparison, so a *correct*
// PIN still burned one of the five slots. Signing in five times inside the
// window therefore locked the user out of their own controller, and the error
// they saw was "Invalid PIN code" for a PIN that was right. Only failures are
// rate-limited now.
func (rs *RemoteServer) recordAuthFailure() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.authAttempts++
}

func (rs *RemoteServer) isAuthorized(r *http.Request) bool {
	rs.mu.RLock()
	validToken := rs.authToken
	rs.mu.RUnlock()

	if validToken == "" {
		return false
	}

	// 1. Check Header
	if reqToken := r.Header.Get("X-NexVPN-Token"); secureEqual(reqToken, validToken) {
		rs.touchActivity()
		return true
	}
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		if secureEqual(strings.TrimPrefix(authHeader, "Bearer "), validToken) {
			rs.touchActivity()
			return true
		}
	}

	// Tokens must not be accepted in URLs: URLs are commonly logged and leaked via referrers.
	return false
}

func (rs *RemoteServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PIN == "" {
		http.Error(w, `{"error":"Missing PIN"}`, http.StatusBadRequest)
		return
	}

	rs.mu.RLock()
	validPin := rs.authPin
	token := rs.authToken
	rs.mu.RUnlock()

	// The rate limit is checked first so a locked-out client cannot even reach
	// the comparison, but the attempt is only counted when the PIN is wrong.
	if !rs.authAllowed() {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Too many attempts. Try again shortly.",
		})
		return
	}

	if !secureEqual(strings.TrimSpace(req.PIN), validPin) {
		rs.recordAuthFailure()
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid PIN code",
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
	})
}

func (rs *RemoteServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if rs.engine == nil || rs.store == nil {
		http.Error(w, `{"error":"remote service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	if !rs.isAuthorized(r) {
		http.Error(w, `{"error":"Unauthorized access - PIN required"}`, http.StatusUnauthorized)
		return
	}

	status := rs.engine.GetStatus()
	stats := rs.engine.GetStats()
	rawProfiles := rs.store.GetProfiles()

	// Sanitize profiles to prevent credential and private key leakage over LAN
	type RemoteProfileSummary struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Protocol  string `json:"protocol"`
		Category  string `json:"category"`
		Group     string `json:"group"`
		LatencyMs int    `json:"latency_ms"`
	}

	sanitizedProfiles := make([]RemoteProfileSummary, 0, len(rawProfiles))
	for _, p := range rawProfiles {
		sanitizedProfiles = append(sanitizedProfiles, RemoteProfileSummary{
			ID:        p.ID,
			Name:      p.Name,
			Protocol:  string(p.Protocol),
			Category:  string(p.Category),
			Group:     p.Group,
			LatencyMs: p.LatencyMs,
		})
	}

	resp := map[string]interface{}{
		"status":   status,
		"stats":    stats,
		"profiles": sanitizedProfiles,
		"time":     time.Now().Unix(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (rs *RemoteServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if rs.engine == nil || rs.store == nil {
		http.Error(w, `{"error":"remote service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	if !rs.isAuthorized(r) {
		http.Error(w, `{"error":"Unauthorized access"}`, http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProfileID string `json:"profile_id"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProfileID == "" {
		http.Error(w, `{"error":"Missing profile_id"}`, http.StatusBadRequest)
		return
	}

	profile, err := rs.store.GetProfile(req.ProfileID)
	if err != nil {
		http.Error(w, `{"error":"Profile not found"}`, http.StatusNotFound)
		return
	}

	settings := rs.store.GetSettings()
	err = rs.engine.Connect(profile, settings)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "profile": profile.Name})
}

func (rs *RemoteServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if rs.engine == nil {
		http.Error(w, `{"error":"remote service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	if !rs.isAuthorized(r) {
		http.Error(w, `{"error":"Unauthorized access"}`, http.StatusUnauthorized)
		return
	}

	// Disconnecting is a state change, so it is POST-only, exactly like
	// /api/connect. Without this a GET (which any browser, prefetcher or image
	// tag can issue, and which carries no CSRF protection) would tear the tunnel
	// down — the token would have to leak first, but a state change must never be
	// reachable by a safe method.
	if r.Method != "POST" {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	err := rs.engine.Disconnect()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// setSecurityHeaders applies defensive headers to every response the LAN
// controller serves. The UI is entirely self-contained (inline CSS/JS, no
// external assets), so the CSP can be maximally strict.
func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	// 'unsafe-inline' is required because the page inlines its own style and
	// script blocks; everything is same-origin and there are no remote loads.
	h.Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; "+
			"connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
}

// securityHeadersMiddleware guarantees every handler - including ones added
// later - carries the defensive headers without each having to remember.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		next.ServeHTTP(w, r)
	})
}

func (rs *RemoteServer) handleWebUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
<title>NexVPN Mobile Controller</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  body { background: #0a0e17; color: #e2e8f0; min-height: 100vh; display: flex; flex-direction: column; padding: 16px; }
  .card { background: #131b2e; border: 1px solid #1e293b; border-radius: 24px; padding: 24px; margin-bottom: 16px; text-align: center; box-shadow: 0 10px 30px rgba(0,0,0,0.5); }
  .title { font-size: 20px; font-weight: 800; color: #38bdf8; margin-bottom: 4px; display: flex; align-items: center; justify-content: center; gap: 8px; }
  .subtitle { font-size: 12px; color: #94a3b8; margin-bottom: 24px; }
  .orb { width: 140px; height: 140px; border-radius: 50%; margin: 0 auto 20px; display: flex; align-items: center; justify-content: center; font-size: 16px; font-weight: bold; border: 3px solid #334155; transition: all 0.3s; cursor: pointer; user-select: none; }
  .orb.connected { background: radial-gradient(circle, #059669, #022c22); border-color: #10b981; box-shadow: 0 0 40px rgba(16, 185, 129, 0.4); color: white; }
  .orb.disconnected { background: radial-gradient(circle, #1e293b, #0f172a); border-color: #475569; color: #94a3b8; }
  .orb.connecting { background: radial-gradient(circle, #d97706, #451a03); border-color: #f59e0b; color: white; animation: pulse 1.5s infinite; }
  @keyframes pulse { 0%, 100% { transform: scale(1); } 50% { transform: scale(1.05); } }
  .stats-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 16px; }
  .stat-box { background: #0f172a; padding: 12px; border-radius: 16px; border: 1px solid #1e293b; }
  .stat-label { font-size: 11px; color: #64748b; font-weight: 600; text-transform: uppercase; }
  .stat-val { font-size: 15px; font-weight: 800; color: #f8fafc; margin-top: 4px; font-family: monospace; }
  select { width: 100%; background: #0f172a; color: white; border: 1px solid #334155; padding: 14px; border-radius: 16px; font-size: 13px; font-weight: 600; margin-top: 16px; outline: none; }
  .btn { width: 100%; padding: 14px; border-radius: 16px; border: none; font-weight: bold; font-size: 14px; cursor: pointer; transition: 0.2s; margin-top: 12px; }
  .btn-primary { background: #0284c7; color: white; }
  .btn-primary:active { background: #0369a1; }
  .pin-input { width: 100%; padding: 16px; font-size: 24px; text-align: center; letter-spacing: 8px; border-radius: 16px; border: 2px solid #38bdf8; background: #0f172a; color: #38bdf8; font-weight: 800; outline: none; margin: 16px 0; }
  .auth-lock { display: block; }
  .main-app { display: none; }
</style>
</head>
<body>
  <!-- PIN Authentication Screen -->
  <div id="auth-screen" class="card auth-lock">
    <div class="title">🔒 NexVPN Authentication</div>
    <div class="subtitle">Enter the 6-Digit PIN shown on your desktop screen</div>
    <div style="color:#fbbf24;font-size:11px;margin-bottom:12px;line-height:1.5;">This page is reachable by every device on your local network. Keep your PIN private and stop the service from the desktop app when you are done.</div>
    <input id="pin-field" type="number" pattern="[0-9]*" class="pin-input" placeholder="000000" maxlength="6" autofocus />
    <button class="btn btn-primary" onclick="submitPIN()">Unlock Controller</button>
    <div id="auth-error" style="color: #f43f5e; font-size: 12px; margin-top: 12px; font-weight: 600;"></div>
  </div>

  <!-- Main Controller Screen -->
  <div id="main-screen" class="main-app">
    <div class="card">
      <div class="title">⚡ NexVPN Remote</div>
      <div class="subtitle">Secure Mobile Controller</div>
      
      <div id="orb" class="orb disconnected" onclick="toggleConnect()">
        <span id="orb-text">CONNECT</span>
      </div>

      <div id="active-profile" style="font-size: 14px; font-weight: 700; color: #e2e8f0; margin-bottom: 4px;">Disconnected</div>
      <div id="active-server" style="font-size: 11px; color: #64748b; font-family: monospace;">-</div>

      <div class="stats-grid">
        <div class="stat-box">
          <div class="stat-label">⬇ Download</div>
          <div id="stat-down" class="stat-val">0 B</div>
        </div>
        <div class="stat-box">
          <div class="stat-label">⬆ Upload</div>
          <div id="stat-up" class="stat-val">0 B</div>
        </div>
        <div class="stat-box">
          <div class="stat-label">⚡ Speed</div>
          <div id="stat-speed" class="stat-val">0 KB/s</div>
        </div>
        <div class="stat-box">
          <div class="stat-label">⏱ Duration</div>
          <div id="stat-dur" class="stat-val">00:00</div>
        </div>
      </div>
    </div>

    <div class="card" style="padding: 16px;">
      <div style="font-size: 12px; font-weight: 700; color: #94a3b8; text-align: left;">SWITCH SERVER</div>
      <select id="server-select"></select>
      <button class="btn btn-primary" onclick="switchServer()">Connect Selected</button>
    </div>
  </div>

<script>
let currentStatus = 'disconnected';
let profiles = [];let authToken = localStorage.getItem('nex_auth_token') || '';
// Tokens are intentionally accepted only in request headers. Query-string tokens
// are not imported because URLs can leak through browser history and referrers.

function showMain() {
  document.getElementById('auth-screen').style.display = 'none';
  document.getElementById('main-screen').style.display = 'block';
  updateState();
  setInterval(updateState, 1500);
}

async function submitPIN() {
  const pin = document.getElementById('pin-field').value.trim();
  const errDiv = document.getElementById('auth-error');
  errDiv.innerText = '';
  
  if (!pin || pin.length < 4) {
    errDiv.innerText = 'Please enter a valid PIN code';
    return;
  }

  try {
    const res = await fetch('/api/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pin })
    });
    const data = await res.json();
    if (data.success && data.token) {
      authToken = data.token;
      localStorage.setItem('nex_auth_token', authToken);
      showMain();
    } else {
      errDiv.innerText = data.error || 'Invalid PIN code';
    }
  } catch (err) {
    errDiv.innerText = 'Connection to desktop failed';
  }
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDuration(sec) {
  if (!sec) return '00:00';
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
}

async function updateState() {
  if (!authToken) return;
  try {
    const res = await fetch('/api/status', {
      headers: { 'X-NexVPN-Token': authToken }
    });
    if (res.status === 401) {
      localStorage.removeItem('nex_auth_token');
      authToken = '';
      document.getElementById('auth-screen').style.display = 'block';
      document.getElementById('main-screen').style.display = 'none';
      return;
    }
    const data = await res.json();
    currentStatus = data.status.state;
    
    const orb = document.getElementById('orb');
    const orbText = document.getElementById('orb-text');
    orb.className = 'orb ' + currentStatus;
    orbText.innerText = currentStatus.toUpperCase();

    document.getElementById('active-profile').innerText = data.status.profile_name || 'Disconnected';
    document.getElementById('active-server').innerText = data.status.server || '-';
    
    document.getElementById('stat-down').innerText = formatBytes(data.stats.bytes_received);
    document.getElementById('stat-up').innerText = formatBytes(data.stats.bytes_sent);
    document.getElementById('stat-speed').innerText = formatBytes(data.stats.speed_download) + '/s';
    document.getElementById('stat-dur').innerText = formatDuration(data.status.duration);

    if (profiles.length !== data.profiles.length) {
      profiles = data.profiles;
      const select = document.getElementById('server-select');
      select.innerHTML = '';
      profiles.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.id;
        opt.innerText = p.name + (p.latency_ms ? ' (' + p.latency_ms + 'ms)' : '');
        select.appendChild(opt);
      });
    }
  } catch (err) {
    console.error(err);
  }
}

async function toggleConnect() {
  if (currentStatus === 'connected') {
    await fetch('/api/disconnect', { 
      method: 'POST',
      headers: { 'X-NexVPN-Token': authToken }
    });
  } else {
    const select = document.getElementById('server-select');
    if (select.value) {
      await fetch('/api/connect', {
        method: 'POST',
        headers: { 
          'Content-Type': 'application/json',
          'X-NexVPN-Token': authToken
        },
        body: JSON.stringify({ profile_id: select.value })
      });
    }
  }
  updateState();
}

async function switchServer() {
  const select = document.getElementById('server-select');
  if (select.value) {
    await fetch('/api/connect', {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'X-NexVPN-Token': authToken
      },
      body: JSON.stringify({ profile_id: select.value })
    });
    updateState();
  }
}

if (authToken) {
  showMain();
}
</script>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}

// handleSubscription serves all valid profile URI links as a standard base64 subscription
// Protected by token authentication to prevent LAN credential leakage (C4 fix)
func (rs *RemoteServer) handleSubscription(w http.ResponseWriter, r *http.Request) {
	if !rs.isAuthorized(r) {
		http.Error(w, `{"error":"Unauthorized - token required for subscription"}`, http.StatusUnauthorized)
		return
	}
	if rs.store == nil {
		http.Error(w, "store not available", http.StatusInternalServerError)
		return
	}

	profiles := rs.store.GetProfiles()
	var links []string
	for _, p := range profiles {
		if p.RawURI != "" {
			links = append(links, strings.TrimSpace(p.RawURI))
		}
	}

	if len(links) == 0 {
		http.Error(w, "no exportable subscription links available in NexVPN", http.StatusNotFound)
		return
	}

	bundle := strings.Join(links, "\n")
	b64 := base64.StdEncoding.EncodeToString([]byte(bundle))

	var totalTraffic int64
	if rs.engine != nil {
		st := rs.engine.GetStats()
		totalTraffic = st.BytesReceived + st.BytesSent
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="nexvpn_sub.txt"`)
	w.Header().Set("Profile-Update-Interval", "12")
	w.Header().Set("Subscription-Userinfo", fmt.Sprintf("upload=0; download=%d; total=107374182400; expire=%d", totalTraffic, time.Now().Add(30*24*time.Hour).Unix()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b64))
}
