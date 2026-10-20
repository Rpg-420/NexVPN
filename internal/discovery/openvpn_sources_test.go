package discovery

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A minimal but structurally real .ovpn body, the shape VPN Gate publishes.
const sampleOvpnBody = `client
dev tun
proto udp
remote 219.100.37.197 1194
nobind
persist-key
persist-tun
auth-user-pass
<ca>
-----BEGIN CERTIFICATE-----
MIIB
-----END CERTIFICATE-----
</ca>
`

func vpnGateRow(host, ip, country, code, operator, body string) string {
	// HostName,IP,Score,Ping,Speed,CountryLong,CountryShort,NumVpnSessions,
	// Uptime,TotalUsers,TotalTraffic,LogType,Operator,Message,ConfigBase64
	return strings.Join([]string{
		host, ip, "2592822", "15", "178616444", country, code, "86",
		"525037386", "15634899", "889707720781121", "2weeks", operator, "",
		base64.StdEncoding.EncodeToString([]byte(body)),
	}, ",")
}

func vpnGateFixture(rows ...string) string {
	var b strings.Builder
	b.WriteString("*vpn_servers\n")
	b.WriteString("#HostName,IP,Score,Ping,Speed,CountryLong,CountryShort,NumVpnSessions,")
	b.WriteString("Uptime,TotalUsers,TotalTraffic,LogType,Operator,Message,OpenVPN_ConfigData_Base64\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	b.WriteString("*\n")
	return b.String()
}

func resetOpenVPNCache() {
	openVPNCacheMu.Lock()
	openVPNConfigs = map[string]string{}
	openVPNCacheMu.Unlock()
}

func TestParseVPNGateList(t *testing.T) {
	resetOpenVPNCache()
	src, ok := lookupOpenVPNSource("vpngate")
	if !ok {
		t.Fatal("vpngate source missing from the registry")
	}

	csv := vpnGateFixture(
		vpnGateRow("public-vpn-202", "219.100.37.197", "Japan", "JP",
			"Daiyuu Nobori_ Japan. Academic Use Only.", sampleOvpnBody),
		vpnGateRow("vpn256634230", "150.40.105.22", "Korea Republic of", "KR",
			"unknown", sampleOvpnBody),
	)

	servers, err := parseVPNGateList(strings.NewReader(csv), src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}

	first := servers[0]
	if first.ID != "vpg-219.100.37.197" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.IP != "219.100.37.197" {
		t.Errorf("IP = %q", first.IP)
	}
	if first.Country != "Japan" || first.CountryCode != "JP" {
		t.Errorf("country = %q / %q", first.Country, first.CountryCode)
	}
	if first.Score != 2592822 {
		t.Errorf("score = %d", first.Score)
	}
	if first.Sessions != 86 {
		t.Errorf("sessions = %d", first.Sessions)
	}
	if first.SpeedBps != 178616444 {
		t.Errorf("speed = %d", first.SpeedBps)
	}
	if first.Operator == "" {
		t.Error("operator must be carried through so the UI can name who runs the relay")
	}

	// The port comes from the profile's own `remote` line, not from a guess:
	// this fixture says 1194, not the 443 default.
	if first.Port != 1194 {
		t.Errorf("port = %d, want 1194 (from the remote line)", first.Port)
	}

	// The body must be cached, since it is what the import path resolves.
	body, ok := cachedOpenVPNConfig(first.ID)
	if !ok {
		t.Fatal("the .ovpn body was not cached")
	}
	if !strings.Contains(body, "remote 219.100.37.197 1194") {
		t.Errorf("cached body looks wrong: %q", body)
	}
}

// A malformed row must not take the whole catalogue down — a real list always has
// a few rows a stricter parser would choke on.
func TestParseVPNGateListSkipsBadRows(t *testing.T) {
	resetOpenVPNCache()
	src, _ := lookupOpenVPNSource("vpngate")

	csv := vpnGateFixture(
		"too,few,fields",
		vpnGateRow("good", "1.2.3.4", "Japan", "JP", "op", sampleOvpnBody),
		"a,b,c,d,e,f,g,h,i,j,k,l,m,n,!!!not-base64!!!",
		"",
		vpnGateRow("good2", "5.6.7.8", "Germany", "DE", "op", sampleOvpnBody),
	)

	servers, err := parseVPNGateList(strings.NewReader(csv), src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want the 2 good rows: %+v", len(servers), servers)
	}
}

func TestParseVPNGateListRejectsEmptyCatalogue(t *testing.T) {
	resetOpenVPNCache()
	src, _ := lookupOpenVPNSource("vpngate")

	if _, err := parseVPNGateList(strings.NewReader("*vpn_servers\n*\n"), src); err == nil {
		t.Fatal("an empty catalogue must be an error, not an empty success")
	}
}

func TestParseVPNGateListPrefersTheRemoteHost(t *testing.T) {
	resetOpenVPNCache()
	src, _ := lookupOpenVPNSource("vpngate")

	// VPN Gate profiles dial a hostname while the row also carries an IP. The
	// profile's own `remote` line is what the core will actually use.
	body := strings.Replace(sampleOvpnBody, "remote 219.100.37.197 1194", "remote vpn123.opengw.net 443", 1)
	csv := vpnGateFixture(vpnGateRow("row-host", "203.0.113.9", "Japan", "JP", "op", body))

	servers, err := parseVPNGateList(strings.NewReader(csv), src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if servers[0].HostName != "vpn123.opengw.net" {
		t.Errorf("host = %q, want the remote line's host", servers[0].HostName)
	}
	if servers[0].Port != 443 {
		t.Errorf("port = %d, want 443", servers[0].Port)
	}
	// The IP stays available for the reachability probe, which needs an address.
	if servers[0].IP != "203.0.113.9" {
		t.Errorf("IP = %q", servers[0].IP)
	}
}

func TestBuildOpenVPNProfile(t *testing.T) {
	resetOpenVPNCache()
	src, _ := lookupOpenVPNSource("vpngate")

	csv := vpnGateFixture(vpnGateRow("public-vpn-202", "219.100.37.197", "Japan", "JP", "op", sampleOvpnBody))
	servers, err := parseVPNGateList(strings.NewReader(csv), src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	profile, err := BuildOpenVPNProfile(servers[0], "", "", "")
	if err != nil {
		t.Fatalf("BuildOpenVPNProfile failed: %v", err)
	}

	if profile.Protocol != "openvpn" {
		t.Errorf("protocol = %q", profile.Protocol)
	}
	if profile.Category != "openvpn" {
		t.Errorf("category = %q", profile.Category)
	}
	if profile.Server != "219.100.37.197" || profile.Port != 1194 {
		t.Errorf("endpoint = %s:%d", profile.Server, profile.Port)
	}
	if !strings.Contains(profile.ConfigContent, "client") {
		t.Errorf("profile carries no .ovpn body")
	}
	if profile.Group != "VPN Gate" {
		t.Errorf("group = %q, want the default", profile.Group)
	}
	// The flag is no longer baked into the name as a regional-indicator emoji:
	// no emoji font is embedded in the app, so it rendered as an empty box. The
	// profile carries CountryCode instead and the UI draws a flag sprite from it.
	if strings.Contains(profile.Name, "🇯🇵") {
		t.Errorf("name should be plain text now, got %q", profile.Name)
	}
	if !strings.Contains(profile.Name, "Japan") {
		t.Errorf("name should carry the country name: %q", profile.Name)
	}
	if profile.CountryCode != "JP" {
		t.Errorf("country_code = %q, want JP so the UI can render a flag", profile.CountryCode)
	}
	if profile.Country == "" {
		t.Error("country should be populated for display")
	}
	if profile.ID == "" {
		t.Error("profile needs an ID")
	}
}

// A cache miss must be reported, never papered over with an empty profile — an
// empty ConfigContent would import a profile that cannot connect and give the
// user no idea why.
func TestBuildOpenVPNProfileWithoutCacheEntry(t *testing.T) {
	resetOpenVPNCache()

	_, err := BuildOpenVPNProfile(OpenVPNServer{ID: "vpg-nothing", IP: "1.2.3.4", Port: 443}, "", "", "")
	if err == nil {
		t.Fatal("expected an error for a server that was never fetched")
	}
	if !strings.Contains(err.Error(), "refresh") {
		t.Errorf("the error should tell the user to refresh the list, got: %v", err)
	}
}

func TestCountryFlag(t *testing.T) {
	cases := map[string]string{
		"JP": "🇯🇵",
		"de": "🇩🇪",
		"US": "🇺🇸",
		"":   "",
		"X":  "",
		"12": "",
	}
	for code, want := range cases {
		if got := countryFlag(code); got != want {
			t.Errorf("countryFlag(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestListOpenVPNSources(t *testing.T) {
	sources := ListOpenVPNSources()
	if len(sources) == 0 {
		t.Fatal("the registry is empty")
	}
	for _, s := range sources {
		if s.ID == "" || s.Name == "" || s.URL == "" {
			t.Errorf("source is missing identifying fields: %+v", s)
		}
		if s.Operator == "" {
			t.Errorf("source %q must name who operates the servers", s.ID)
		}
	}

	// A caller must not be able to mutate the registry through the returned slice.
	sources[0].ID = "tampered"
	if ListOpenVPNSources()[0].ID == "tampered" {
		t.Error("ListOpenVPNSources handed out its backing array")
	}
}

func TestFetchOpenVPNServersRejectsUnknownSource(t *testing.T) {
	if _, err := FetchOpenVPNServers(context.Background(), "nope", ""); err == nil {
		t.Fatal("an unknown source must be an error")
	}
}

// ── the transport the catalogue is fetched with ─────────────────────────────────

// The bug this guards: a blanket `http.Client.Timeout` bounds connect + TLS +
// headers + the whole body as one number, and the VPN Gate list is ~1.35 MB of
// chunked CSV that takes 8-20 s through a proxy and over 40 s on a direct
// international route. A 25-second value expired mid-body on working
// connections and Go blamed the headers phase, which had already finished.
//
// So: no blanket timeout, and a real per-phase deadline for each phase that can
// hang. If someone reinstates `Timeout:`, this fails.
func TestCatalogueClientHasNoBlanketTimeout(t *testing.T) {
	client := catalogueHTTPClient()

	if client.Timeout != 0 {
		t.Errorf("catalogueHTTPClient sets a blanket Timeout of %s; that is what cut a "+
			"working 1.35 MB download short and reported it as a headers failure", client.Timeout)
	}

	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport so the phase timeouts below apply", client.Transport)
	}
	if tr.DialContext == nil {
		t.Error("no DialContext: a host with a black-holed route would hang forever")
	}
	if tr.TLSHandshakeTimeout <= 0 {
		t.Error("no TLSHandshakeTimeout")
	}
	if tr.ResponseHeaderTimeout <= 0 {
		t.Error("no ResponseHeaderTimeout: a host that accepts and then says nothing would hang forever")
	}
}

// Replacing http.DefaultTransport with a custom one silently drops proxy
// support unless it is set explicitly, and on a filtered line a local proxy is
// often the only route to this host — measured here it was the difference
// between 15 s and 40 s for the same file. Asserting presence rather than
// behaviour because Go caches the proxy environment once per process, so a test
// cannot reliably change it after the fact.
func TestCatalogueClientKeepsProxySupport(t *testing.T) {
	tr := catalogueHTTPClient().Transport.(*http.Transport)
	if tr.Proxy == nil {
		t.Fatal("the transport has no Proxy function, so HTTP_PROXY / HTTPS_PROXY are ignored")
	}
}

// A single reset or a stalled body is common on a long route, so the fetch is
// tried twice. The second attempt has to be a genuinely fresh request: reusing
// one whose connection was torn down is not reliable.
func TestFetchCatalogueRetriesOnce(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// Kill the connection without a response.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Error("test server cannot hijack")
				return
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = io.WriteString(w, vpnGateFixture(
			vpnGateRow("good", "1.2.3.4", "Japan", "JP", "op", sampleOvpnBody)))
	}))
	defer srv.Close()

	resp, err := fetchCatalogue(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("the retry did not recover from a dropped first connection: %v", err)
	}
	_ = resp.Body.Close()

	if attempts != 2 {
		t.Errorf("the server saw %d attempts, want 2", attempts)
	}
}

// A host that accepts the connection and then never answers must produce an
// error that names the remedy, not a bare transport string. The phase timeout is
// compressed so the test does not take 45 seconds.
func TestFetchCatalogueTimeoutNamesTheRemedy(t *testing.T) {
	shrinkCatalogueTimeouts(t, 300*time.Millisecond)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept, send nothing, and wait for the client to give up.
		<-r.Context().Done()
	}))
	defer srv.Close()

	src, _ := lookupOpenVPNSource("vpngate")
	_, err := FetchOpenVPNServers(context.Background(), "vpngate", srv.URL)
	if err == nil {
		t.Fatal("a server that never sends headers must fail")
	}

	msg := err.Error()
	if !strings.Contains(msg, "timed out") {
		t.Errorf("the error should say it timed out, got: %v", err)
	}
	// The actionable half: what the user can do about it.
	if !strings.Contains(msg, "mirror") {
		t.Errorf("the error should mention the mirror field, got: %v", err)
	}
	if !strings.Contains(msg, src.Name) {
		t.Errorf("the error should name the source, got: %v", err)
	}
}

// A mirror that streams forever must not be read forever. The cap is compressed
// so the test does not have to push 32 MB.
func TestFetchCatalogueEnforcesTheSizeCap(t *testing.T) {
	shrinkCatalogueCap(t, 4096)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Well past the cap, and structured so a truncated read would still
		// parse into plausible-looking rows.
		for i := 0; i < 200; i++ {
			_, _ = io.WriteString(w, vpnGateRow("h", "1.2.3.4", "Japan", "JP", "op", sampleOvpnBody)+"\n")
		}
	}))
	defer srv.Close()

	_, err := FetchOpenVPNServers(context.Background(), "vpngate", srv.URL)
	if err == nil {
		t.Fatal("an oversized list must be refused rather than silently truncated")
	}
	if !strings.Contains(err.Error(), "safety limit") {
		t.Errorf("the error should say the size limit was hit, got: %v", err)
	}
}

// TestFetchVPNGateLive exercises the real endpoint with the real deadlines.
//
// Opt-in, because it moves ~1.35 MB and takes 8-20 s through a proxy and over
// 40 s on a direct international route — far too slow for CI, and dependent on
// a third party being up. It exists because the whole point of this change is
// that the list arrives at all, and only the live endpoint can show that.
//
//	NEXVPN_LIVE_CATALOGUE=1 go test ./internal/discovery/ -run Live -v
func TestFetchVPNGateLive(t *testing.T) {
	if os.Getenv("NEXVPN_LIVE_CATALOGUE") == "" {
		t.Skip("set NEXVPN_LIVE_CATALOGUE=1 to fetch the real VPN Gate list")
	}

	resetOpenVPNCache()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	start := time.Now()
	servers, err := FetchOpenVPNServers(ctx, "vpngate", "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("live fetch failed after %s: %v", elapsed.Round(time.Second), err)
	}

	t.Logf("fetched %d relays in %s", len(servers), elapsed.Round(time.Second))
	if len(servers) < 10 {
		t.Errorf("only %d relays came back; the live list is normally ~100", len(servers))
	}

	// The bodies must be cached, or the import path cannot resolve them.
	body, ok := cachedOpenVPNConfig(servers[0].ID)
	if !ok || !strings.Contains(body, "remote ") {
		t.Fatalf("the first relay's .ovpn body was not cached: ok=%v len=%d", ok, len(body))
	}
	if !strings.Contains(body, "<ca>") {
		t.Error("the relay profile carries no inline certificate")
	}
}

// shrinkCatalogueTimeouts compresses the phase deadlines and returns a restore
// function, so a test can exercise a timeout without waiting for the real one.
func shrinkCatalogueTimeouts(t *testing.T, d time.Duration) {
	t.Helper()
	oldDial, oldTLS, oldHeader := catalogueDialTimeout, catalogueTLSHandshakeTime, catalogueHeaderTimeout
	catalogueDialTimeout, catalogueTLSHandshakeTime, catalogueHeaderTimeout = d, d, d
	// Registered rather than returned for the caller to `defer`: these are
	// package-level values, so a test that returns early — or fails — without
	// restoring them leaves every later test in the package running against
	// millisecond deadlines and a 4 KB size cap.
	t.Cleanup(func() {
		catalogueDialTimeout, catalogueTLSHandshakeTime, catalogueHeaderTimeout = oldDial, oldTLS, oldHeader
	})
}

func shrinkCatalogueCap(t *testing.T, n int64) {
	t.Helper()
	old := maxCatalogueBytes
	maxCatalogueBytes = n
	t.Cleanup(func() { maxCatalogueBytes = old })
}
