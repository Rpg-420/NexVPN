// Public OpenVPN server catalogues.
//
// The idea comes from MLMVPN, which ships a folder of ready-made OpenVPN
// profiles next to its binary. NexVPN already had a free-config aggregator
// (free_configs.go) but it only understood the proxy family — vless, vmess,
// trojan, shadowsocks, hysteria2, tuic, wireguard. There was no equivalent for
// OpenVPN, so the one protocol with a genuinely public server pool was the one
// the app could not draw from.
//
// This file adds a *catalogue* layer: a provider registry plus the parsers that
// turn a published list into importable profiles.
//
// # What is deliberately not here
//
// MLMVPN also redistributes 47 TunnelBear .ovpn files plus TunnelBear's CA
// certificate. Those are a commercial provider's client configs, handed to its
// own account holders; re-shipping them would let anyone ride a paid service
// without an account. Nothing in this file bundles a third-party provider's
// configs or credentials.
//
// TunnelBear still works with NexVPN, through the ordinary path: download your
// own .ovpn from your TunnelBear account and import it (ImportOpenVPNProfile
// takes the username/password the file's `auth-user-pass` directive asks for).
// The catalogue below is for sources that publish their servers for public use.
package discovery

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
)

// OpenVPNSourceInfo describes a catalogue NexVPN can pull server rows from.
type OpenVPNSourceInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	URL           string `json:"url"`
	Operator      string `json:"operator"`
	License       string `json:"license"`
	NeedsCreds    bool   `json:"needs_credentials"`
	DefaultPort   int    `json:"default_port"`
	SupportsProbe bool   `json:"supports_probe"`
}

// OpenVPNServer is one row of a catalogue.
//
// The .ovpn body is deliberately absent: see openVPNConfigCache below.
type OpenVPNServer struct {
	ID          string `json:"id"`
	SourceID    string `json:"source_id"`
	HostName    string `json:"host_name"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Operator    string `json:"operator"`
	Score       int    `json:"score"`
	PingMs      int    `json:"ping_ms"`
	SpeedBps    int64  `json:"speed_bps"`
	Sessions    int    `json:"sessions"`
	LatencyMs   int    `json:"latency_ms"`
	IsAlive     bool   `json:"is_alive"`
}

// OpenVPNServerTestProgress mirrors FreeConfigTestProgress so the UI can reuse
// the same progress widget.
type OpenVPNServerTestProgress struct {
	TestedCount int             `json:"tested_count"`
	TotalCount  int             `json:"total_count"`
	Current     string          `json:"current"`
	Results     []OpenVPNServer `json:"results"`
}

// openVPNSources is the registry. Adding a provider means adding an entry here
// plus a parser branch in FetchOpenVPNServers.
var openVPNSources = []OpenVPNSourceInfo{
	{
		ID:   "vpngate",
		Name: "VPN Gate (University of Tsukuba)",
		Description: "A public relay pool run as an academic experiment by the " +
			"University of Tsukuba. Every relay is published with the same shared " +
			"client certificate, so the profiles work without an account. " +
			"Availability and speed vary per relay and it is not a privacy service: " +
			"relays are operated by volunteers, and the project asks that you do not " +
			"use it for anything you would not want logged.",
		URL:           "https://www.vpngate.net/api/iphone/",
		Operator:      "University of Tsukuba (Japan)",
		License:       "Public relay pool, published by the operator for free use",
		NeedsCreds:    false,
		DefaultPort:   443,
		SupportsProbe: true,
	},
}

// ListOpenVPNSources returns the provider registry.
func ListOpenVPNSources() []OpenVPNSourceInfo {
	out := make([]OpenVPNSourceInfo, len(openVPNSources))
	copy(out, openVPNSources)
	return out
}

// lookupOpenVPNSource resolves a source ID to its definition.
func lookupOpenVPNSource(id string) (OpenVPNSourceInfo, bool) {
	for _, s := range openVPNSources {
		if s.ID == id {
			return s, true
		}
	}
	return OpenVPNSourceInfo{}, false
}

// openVPNConfigCache holds the .ovpn bodies of the most recent catalogue fetch,
// keyed by server ID.
//
// They are kept server-side rather than returned to the UI because each body is
// ~13 KB: a 98-relay catalogue would push well over a megabyte of base64 across
// the JS bridge twice — once on fetch, once again on import. The UI only needs
// the metadata, and the import path looks the body up by ID.
//
// Package-level because the data is public, identical for every App instance,
// and there is exactly one catalogue view at a time; the alternative is a field
// on App that exists solely to hold a cache.
var (
	openVPNCacheMu sync.Mutex
	openVPNConfigs = map[string]string{}
)

func cacheOpenVPNConfig(id, body string) {
	openVPNCacheMu.Lock()
	openVPNConfigs[id] = body
	openVPNCacheMu.Unlock()
}

func cachedOpenVPNConfig(id string) (string, bool) {
	openVPNCacheMu.Lock()
	defer openVPNCacheMu.Unlock()
	body, ok := openVPNConfigs[id]
	return body, ok
}

// How long the catalogue fetch is allowed to spend on each phase, and how many
// times it is tried.
//
// These replace what used to be a single `http.Client{Timeout: 25 * time.Second}`,
// and the reason is worth recording. That field bounds connect + TLS + response
// headers + the *entire body* as one number. The VPN Gate list is not a small
// document: measured against the live endpoint it is ~1.35 MB of chunked CSV —
// the base64 .ovpn body of every relay — sent with `Transfer-Encoding: chunked`
// and therefore no Content-Length, so there is nothing to size a deadline
// against. Measured end-to-end it takes 8-20 s through a local proxy and over
// 40 s on a direct international route.
//
// A 25-second blanket deadline therefore expired *mid-body* on connections that
// were working perfectly, and Go reports every Client.Timeout expiry as
// "(Client.Timeout exceeded while awaiting headers)" — pointing at the one phase
// that had already finished. The phase timeouts below catch a host that is
// genuinely unresponsive, which is what a timeout should mean, while the
// caller's context owns the overall budget.
//
// Variables rather than constants so tests can compress them: exercising the
// timeout and size-limit paths against their real values would mean a 45-second
// test and a 32 MB fixture. Nothing outside this package may write them.
var (
	catalogueDialTimeout      = 15 * time.Second
	catalogueTLSHandshakeTime = 20 * time.Second
	catalogueHeaderTimeout    = 45 * time.Second

	// maxCatalogueBytes bounds what a broken or hostile mirror can make the app
	// read. The real list is ~1.35 MB, so this is roughly 25x headroom: large
	// enough never to bite a legitimate source, small enough that a stream of
	// endless data cannot exhaust memory.
	maxCatalogueBytes int64 = 32 << 20
)

// catalogueAttempts is deliberately 2 and not a loop. A single reset or a
// stalled body is common on a long route and a second try often succeeds; a
// second failure means the path is genuinely unusable, and repeating it only
// keeps the user staring at a spinner.
const catalogueAttempts = 2

// catalogueHTTPClient returns a client sized for a large, slow, chunked
// document. It deliberately sets no http.Client.Timeout — see the note above.
func catalogueHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			// Kept, and load-bearing: on a filtered line a local proxy is often
			// the only route out, and it is a large part of the difference
			// between this fetch succeeding and timing out. Note that Go reads
			// the environment once per process, so setting HTTP_PROXY after the
			// app has started has no effect on it.
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   catalogueDialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   catalogueTLSHandshakeTime,
			ResponseHeaderTimeout: catalogueHeaderTimeout,
			ExpectContinueTimeout: time.Second,
			IdleConnTimeout:       30 * time.Second,
			MaxIdleConns:          4,
		},
	}
}

// FetchOpenVPNServers downloads and parses a catalogue.
//
// overrideURL lets the caller point at a mirror. That is not a nicety: on many
// Iranian connections www.vpngate.net is unreachable — the name resolves to a
// block page and a handshake naming it is reset — so the caller needs a way to
// route the fetch through a relay of their own.
func FetchOpenVPNServers(ctx context.Context, sourceID string, overrideURL string) ([]OpenVPNServer, error) {
	src, ok := lookupOpenVPNSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("unknown OpenVPN source %q", sourceID)
	}

	url := strings.TrimSpace(overrideURL)
	if url == "" {
		url = src.URL
	}

	start := time.Now()
	resp, err := fetchCatalogue(ctx, url)
	if err != nil {
		return nil, describeCatalogueFailure(src, err, ctx, time.Since(start))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s list returned HTTP %d", src.Name, resp.StatusCode)
	}

	// Wrapped rather than handed over directly so a mirror that never stops
	// sending cannot be read forever. `lr.N` is checked afterwards to tell a
	// complete list from one that was cut off at the limit — a silently
	// truncated catalogue would look like a smaller but valid one.
	body := &io.LimitedReader{R: resp.Body, N: maxCatalogueBytes}

	var servers []OpenVPNServer
	switch src.ID {
	case "vpngate":
		servers, err = parseVPNGateList(body, src)
	default:
		return nil, fmt.Errorf("source %q has no parser", src.ID)
	}
	if err != nil {
		return nil, err
	}
	if body.N <= 0 {
		return nil, fmt.Errorf("%s list exceeded the %d MB safety limit and was not used",
			src.Name, maxCatalogueBytes>>20)
	}
	return servers, nil
}

// fetchCatalogue performs the GET, retrying a transport failure once.
//
// The request is rebuilt per attempt rather than reused: a request whose
// context or connection was torn down is not reliably re-sendable, and the cost
// of rebuilding it is nil next to the round trip.
func fetchCatalogue(ctx context.Context, url string) (*http.Response, error) {
	client := catalogueHTTPClient()

	var lastErr error
	for attempt := 1; attempt <= catalogueAttempts; attempt++ {
		if attempt > 1 {
			// A short pause first: an immediate retry against a route that just
			// reset tends to reset again.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		// Accept-Encoding is deliberately NOT set. Go's transport requests gzip
		// on its own and decompresses transparently; setting the header by hand
		// would turn that off and hand the CSV parser raw gzip bytes. The server
		// does honour it, and it roughly halves the transfer.
		req.Header.Set("User-Agent", "NexVPN/1.3")

		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Out of budget: retrying cannot help and would only delay the report.
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// describeCatalogueFailure turns a transport error into something the user can
// act on.
//
// The distinction that matters is "the host never answered" versus "it answered
// and the list did not finish arriving": the first is a dead or blocked route,
// the second is a slow one, and the remedies differ. Go's own wording blurs
// them — it appends "while awaiting headers" to every Client.Timeout expiry,
// including the ones that happened while reading the body — so the two cases are
// separated here instead of being passed through.
func describeCatalogueFailure(src OpenVPNSourceInfo, err error, ctx context.Context, elapsed time.Duration) error {
	const remedy = "If this line is slow or filtered, put a mirror URL in the mirror field, " +
		"or start a local proxy and set HTTP_PROXY / HTTPS_PROXY before launching the app."

	waited := elapsed.Round(time.Second)

	// The context is checked first, and the order matters: an expired deadline
	// arrives as a `net.Error` whose Timeout() is true, so testing that first
	// would report "the server was reached" for a fetch that never got that far.
	if ctx.Err() != nil {
		return fmt.Errorf(
			"the %s list did not arrive within %s. %s (%w)",
			src.Name, waited, remedy, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf(
			"the %s list timed out after %s: the connection was made, but the list (~1.3 MB) "+
				"did not finish arriving. %s (%w)",
			src.Name, waited, remedy, err)
	}
	return fmt.Errorf("%s list is unreachable after %s: %w. %s", src.Name, waited, err, remedy)
}

// vpnGateRemoteRE pulls the endpoint out of a decoded .ovpn body.
var vpnGateRemoteRE = regexp.MustCompile(`(?im)^\s*remote\s+(\S+)\s+(\d+)`)

// parseVPNGateList turns the VPN Gate CSV into catalogue rows and caches the
// decoded profiles.
//
// Layout, verified against the live endpoint: a `*vpn_servers` line, a
// `#HostName,...` header, one row per relay with exactly 15 comma-separated
// fields, then a trailing `*`. The last field is the complete .ovpn, base64
// encoded — VPN Gate hands one shared client certificate to everybody, which is
// what makes each relay's profile usable as-is.
func parseVPNGateList(r io.Reader, src OpenVPNSourceInfo) ([]OpenVPNServer, error) {
	scanner := bufio.NewScanner(r)
	// Relay rows carry a ~13 KB base64 blob, well past the default 64 KB line
	// cap once a row grows; give the scanner room.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var servers []OpenVPNServer
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 15 {
			// A malformed row must not take the whole catalogue down.
			continue
		}

		body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(fields[14]))
		if err != nil || len(body) == 0 {
			continue
		}

		ip := strings.TrimSpace(fields[1])
		host := strings.TrimSpace(fields[0])
		if ip == "" && host == "" {
			continue
		}

		port := src.DefaultPort
		if m := vpnGateRemoteRE.FindSubmatch(body); m != nil {
			if p, err := strconv.Atoi(string(m[2])); err == nil && p > 0 && p <= 65535 {
				port = p
			}
			// Prefer the host the profile actually dials.
			if h := strings.TrimSpace(string(m[1])); h != "" {
				host = h
			}
		}

		id := "vpg-" + ip
		if ip == "" {
			id = "vpg-" + host
		}

		servers = append(servers, OpenVPNServer{
			ID:          id,
			SourceID:    src.ID,
			HostName:    host,
			IP:          ip,
			Port:        port,
			Country:     strings.TrimSpace(fields[5]),
			CountryCode: strings.ToUpper(strings.TrimSpace(fields[6])),
			Operator:    strings.TrimSpace(fields[12]),
			Score:       atoiSafe(fields[2]),
			PingMs:      atoiSafe(fields[3]),
			SpeedBps:    atoi64Safe(fields[4]),
			Sessions:    atoiSafe(fields[7]),
		})
		cacheOpenVPNConfig(id, string(body))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s list: %w", src.Name, err)
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("%s list contained no usable relays", src.Name)
	}

	// Best first: the operator's own score, then relay-reported ping. The list
	// arrives roughly sorted already, but not reliably enough to rely on.
	sort.SliceStable(servers, func(i, j int) bool {
		if servers[i].Score != servers[j].Score {
			return servers[i].Score > servers[j].Score
		}
		return servers[i].PingMs < servers[j].PingMs
	})

	return servers, nil
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func atoi64Safe(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// TestSingleOpenVPNServer measures whether a relay accepts a TCP connection.
//
// A plain dial, not parser.TCPPing: an OpenVPN relay's reachability has nothing
// to do with whether it completes a TLS handshake, and demanding one would mark
// every working relay dead. This answers "is it up?", not "is it unblocked?" —
// only an actual connect can answer the second question.
func TestSingleOpenVPNServer(srv OpenVPNServer, timeout time.Duration) (int, bool) {
	host := srv.IP
	if host == "" {
		host = srv.HostName
	}
	if host == "" {
		return -1, false
	}
	latency := parser.TCPReachable(host, srv.Port, timeout)
	return latency, latency > 0
}

// TestOpenVPNServers probes relays concurrently, reporting progress as it goes.
func TestOpenVPNServers(
	ctx context.Context,
	servers []OpenVPNServer,
	concurrency int,
	timeout time.Duration,
	onProgress func(OpenVPNServerTestProgress),
) []OpenVPNServer {
	if concurrency <= 0 {
		concurrency = 24
	}
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}

	total := len(servers)
	if total == 0 {
		return nil
	}

	tasks := make(chan OpenVPNServer, total)
	resultsChan := make(chan OpenVPNServer, total)

	var wg sync.WaitGroup
	var testedCount int
	var countMu sync.Mutex

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}

				lat, alive := TestSingleOpenVPNServer(item, timeout)
				item.LatencyMs = lat
				item.IsAlive = alive

				countMu.Lock()
				testedCount++
				current := testedCount
				countMu.Unlock()

				resultsChan <- item
				if onProgress != nil {
					onProgress(OpenVPNServerTestProgress{
						TestedCount: current,
						TotalCount:  total,
						Current:     item.Country,
					})
				}
			}
		}()
	}

	for _, s := range servers {
		tasks <- s
	}
	close(tasks)

	wg.Wait()
	close(resultsChan)

	var results []OpenVPNServer
	for r := range resultsChan {
		results = append(results, r)
	}

	// Live relays first, fastest first; the dead sink to the bottom in their
	// original order so the list does not appear to shuffle randomly.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].IsAlive != results[j].IsAlive {
			return results[i].IsAlive
		}
		if !results[i].IsAlive {
			return false
		}
		return results[i].LatencyMs < results[j].LatencyMs
	})

	return results
}

// BuildOpenVPNProfile turns a catalogue row into an importable profile.
//
// The .ovpn body comes from the fetch cache. A cache miss means the catalogue was
// fetched in a different process, or the app was restarted in between — in that
// case the caller must refresh the list, so the error says so rather than
// silently importing an empty profile.
func BuildOpenVPNProfile(srv OpenVPNServer, group, username, password string) (config.Profile, error) {
	body, ok := cachedOpenVPNConfig(srv.ID)
	if !ok || strings.TrimSpace(body) == "" {
		return config.Profile{}, fmt.Errorf(
			"the profile for %s is no longer cached; refresh the server list and try again", srv.ID)
	}

	if group == "" {
		group = "VPN Gate"
	}

	name := srv.Country
	if name == "" {
		name = srv.IP
	}
	// The regional-indicator emoji flag used to be prefixed here. No emoji font
	// is embedded in the app, so it rendered as an empty box in every list. The
	// profile now carries CountryCode and the card draws a real flag sprite from
	// it (see CountryFlag.tsx), so the name stays plain text.
	name = fmt.Sprintf("%s (%s:%d)", name, srv.IP, srv.Port)

	// VPN Gate publishes a shared client certificate, so the profile needs no
	// credentials of its own; the fields stay available for sources that do.
	now := time.Now()
	return config.Profile{
		ID:            "prof-" + strconv.FormatInt(now.UnixNano(), 10) + "-" + srv.ID,
		Name:          name,
		Protocol:      config.ProtocolOpenVPN,
		Category:      config.CategoryOpenVPN,
		ConfigContent: body,
		Server:        srv.IP,
		Port:          srv.Port,
		Group:         group,
		Country:       srv.Country,
		CountryCode:   strings.ToUpper(strings.TrimSpace(srv.CountryCode)),
		Username:      username,
		Password:      password,
		Tags:          []string{"VPNGate", srv.CountryCode},
		LatencyMs:     srv.LatencyMs,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// countryFlag renders an ISO 3166-1 alpha-2 code as its regional-indicator
// emoji, so the catalogue reads the same way the rest of the profile list does.
func countryFlag(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return ""
	}
	const base = 0x1F1E6
	runes := []rune{
		base + rune(code[0]-'A'),
		base + rune(code[1]-'A'),
	}
	for _, r := range runes {
		if r < base || r > base+25 {
			return ""
		}
	}
	return string(runes)
}
