package main

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/discovery"
	"nexvpn/internal/protocols"
)

// TunnelBear, in the OpenVPN section.
//
// TunnelBear's OpenVPN service is documented for manual setup: the operator
// publishes one endpoint per country and the user signs in with their own
// account. That is what this adds — a place to save that account once, a list
// of the operator's countries, and a connect path that carries the connection
// in a way that survives the network it is most needed on.
//
// ## Why the connect path is intercepted rather than the profile stored ready
//
// A working TunnelBear connection needs three things that only exist at connect
// time: the relay's loopback port (ephemeral, and different every time), the
// addresses the operator's hostname currently resolves to, and the account the
// user has selected *now*. Baking any of them into the stored profile would
// make it stale the moment the relay restarts or the user switches accounts.
//
// So the stored profile holds the operator's own published form — `remote
// <host> 443`, UDP — which is a valid profile that works from an unfiltered
// network and is meaningful to anyone reading it. At connect time it is
// rewritten in memory, on a copy, and only for the duration of the call.

var (
	tunnelBearRelayMu sync.Mutex
	tunnelBearRelay   *protocols.SplitRelay
)

// closeTunnelBearRelay stops the relay carrying the current connection.
//
// Called on disconnect and before starting a new one. A relay whose profile has
// been replaced is not merely useless, it is a listener holding a loopback port
// and a socket to a server the user has moved on from.
func closeTunnelBearRelay() {
	tunnelBearRelayMu.Lock()
	relay := tunnelBearRelay
	tunnelBearRelay = nil
	tunnelBearRelayMu.Unlock()

	if relay != nil {
		_ = relay.Close()
	}
}

var tunnelBearRemoteRE = regexp.MustCompile(`(?im)^\s*remote\s+(\S+)`)

// tunnelBearHostOf reports the operator endpoint a profile dials, or "".
//
// The stored `Server` field is the fast path, but a profile the user built by
// hand from a downloaded `.ovpn` may have no `Server` at all, so the config text
// is checked too. Anything that resolves to the operator's domain gets the
// account and the relay; anything else is left alone.
func tunnelBearHostOf(p config.Profile) string {
	if discovery.IsTunnelBearHost(p.Server) {
		return strings.TrimSpace(p.Server)
	}
	for _, m := range tunnelBearRemoteRE.FindAllStringSubmatch(p.ConfigContent, -1) {
		if discovery.IsTunnelBearHost(m[1]) {
			return m[1]
		}
	}
	return ""
}

// resolveTunnelBearHost returns the IPv4 addresses an endpoint currently
// resolves to.
//
// IPv4 only, deliberately: the config pins each of these with a
// `route <ip> 255.255.255.255 net_gateway` line, and `net_gateway` is an IPv4
// concept. Handing an IPv6 address to the relay would produce a profile whose
// relay socket gets captured by the tunnel it is trying to build.
func resolveTunnelBearHost(host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, fmt.Errorf("could not resolve %s: %w", host, err)
	}

	seen := make(map[string]bool, len(addrs))
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		ip := a.String()
		if seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s resolved to no IPv4 address", host)
	}
	return out, nil
}

// applyTunnelBearConnection returns the connect-time view of a stored profile:
// the relay-shaped config plus the account's credentials, and nothing else
// changed.
//
// The copy is the point. The stored profile holds the operator's published
// form and no password; this value holds an ephemeral loopback port and a
// plaintext secret. They must never be the same variable, because the caller
// persists the former after a successful connect and the latter would put both
// the dead port and the password into config.json.
func applyTunnelBearConnection(stored config.Profile, configText, username, password string) config.Profile {
	connect := stored
	connect.ConfigContent = configText
	connect.Username = username
	connect.Password = password
	return connect
}

// prepareTunnelBearProfile rewrites a TunnelBear profile into one that can
// actually connect from here, and reports the account it signed in with.
//
// It returns the profile unchanged and an error when the connection cannot be
// prepared; the caller surfaces the error rather than dialling something that
// is known not to work.
func (a *App) prepareTunnelBearProfile(profile config.Profile) (config.Profile, string, error) {
	host := tunnelBearHostOf(profile)
	if host == "" {
		return profile, "", fmt.Errorf("this profile does not point at a TunnelBear endpoint")
	}

	accountID, username, password, err := discovery.ActiveTunnelBearCredentials()
	if err != nil {
		return profile, "", err
	}

	ips, err := resolveTunnelBearHost(host)
	if err != nil {
		return profile, "", err
	}

	// A relay from a previous connection is stopped first: the port it held is
	// about to be reissued, and two listeners would race for it.
	closeTunnelBearRelay()

	relay := protocols.NewSplitRelay(ips, discovery.TunnelBearTCPPort, func(level, msg, source string) {
		if a.engine != nil {
			a.engine.AddLog(level, msg, source)
		}
	})
	port, err := relay.Start()
	if err != nil {
		return profile, "", fmt.Errorf("could not start the TunnelBear relay: %w", err)
	}

	tunnelBearRelayMu.Lock()
	tunnelBearRelay = relay
	tunnelBearRelayMu.Unlock()

	profile = applyTunnelBearConnection(profile,
		discovery.BuildTunnelBearConfig(host, port, ips), username, password)

	diary.Log("connect", "tunnelbear-prepare",
		"host", host,
		"account", accountID,
		"relay_port", fmt.Sprintf("%d", port),
		"addresses", fmt.Sprintf("%d", len(ips)))

	if a.engine != nil {
		a.engine.AddLog("info", fmt.Sprintf(
			"TunnelBear: %s via the loopback relay on 127.0.0.1:%d (%d address(es), TCP %d)",
			host, port, len(ips), discovery.TunnelBearTCPPort), "openvpn")
	}

	return profile, accountID, nil
}

// ── bound methods ───────────────────────────────────────────────────────────────

// ListTunnelBearCountries returns the operator's published endpoint list.
func (a *App) ListTunnelBearCountries() []discovery.TunnelBearCountry {
	return discovery.ListTunnelBearCountries()
}

// ListTunnelBearAccounts returns the saved accounts, never their passwords.
func (a *App) ListTunnelBearAccounts() []discovery.TunnelBearAccountInfo {
	return discovery.ListTunnelBearAccounts()
}

// TunnelBearSecretBackend names how passwords are stored, so the UI can say
// which protection is actually in force rather than implying one.
func (a *App) TunnelBearSecretBackend() string {
	return discovery.TunnelBearSecretBackend()
}

// SaveTunnelBearAccount adds an account, or updates the one with this ID.
// The password is sealed before it touches the disk and is never read back out.
func (a *App) SaveTunnelBearAccount(id string, username string, password string) (string, error) {
	savedID, err := discovery.SaveTunnelBearAccount(id, username, password)
	if err != nil {
		return "", err
	}
	if a.engine != nil {
		a.engine.AddLog("info", fmt.Sprintf("TunnelBear account %q saved", strings.TrimSpace(username)), "openvpn")
	}
	return savedID, nil
}

// DeleteTunnelBearAccount removes an account and its stored password.
func (a *App) DeleteTunnelBearAccount(id string) error {
	return discovery.DeleteTunnelBearAccount(id)
}

// SetActiveTunnelBearAccount chooses the account new connections sign in with.
func (a *App) SetActiveTunnelBearAccount(id string) error {
	return discovery.SetActiveTunnelBearAccount(id)
}

// TestTunnelBearCountry measures whether an endpoint answers at all.
//
// It dials the operator's TCP port directly, without the relay: the question
// here is "is this country reachable from this machine", and the relay exists
// to make the *handshake* survive, not to make the address reachable. A country
// that fails this is worth skipping; one that passes still needs the relay.
func (a *App) TestTunnelBearCountry(countryID string) (int, error) {
	country, ok := discovery.LookupTunnelBearCountry(countryID)
	if !ok {
		return -1, fmt.Errorf("unknown TunnelBear country %q", countryID)
	}

	ips, err := resolveTunnelBearHost(country.Host)
	if err != nil {
		return -1, err
	}

	best := -1
	for _, ip := range ips {
		start := time.Now()
		conn, dialErr := net.DialTimeout("tcp",
			net.JoinHostPort(ip, fmt.Sprintf("%d", discovery.TunnelBearTCPPort)), 3*time.Second)
		if dialErr != nil {
			continue
		}
		_ = conn.Close()
		elapsed := int(time.Since(start).Milliseconds())
		if elapsed <= 0 {
			elapsed = 1
		}
		if best < 0 || elapsed < best {
			best = elapsed
		}
	}

	if best < 0 {
		return -1, fmt.Errorf("%s answered on none of its %d addresses", country.Host, len(ips))
	}
	return best, nil
}

// ImportTunnelBearProfile adds one country as an OpenVPN profile.
//
// An account must already be saved: a TunnelBear profile without one cannot
// connect, and adding it anyway would put a profile in the list whose only
// possible outcome is an authentication failure.
func (a *App) ImportTunnelBearProfile(countryID string) (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialized")
	}

	country, ok := discovery.LookupTunnelBearCountry(countryID)
	if !ok {
		return "", fmt.Errorf("unknown TunnelBear country %q", countryID)
	}

	if _, _, _, err := discovery.ActiveTunnelBearCredentials(); err != nil {
		return "", fmt.Errorf("save a TunnelBear account first — %w", err)
	}

	name := fmt.Sprintf("TunnelBear %s", country.Name)
	profile := config.Profile{
		Name: name,
		// Stored in the operator's own published form. The relay rewrite happens
		// at connect time; see prepareTunnelBearProfile.
		Protocol:      config.ProtocolOpenVPN,
		Category:      config.CategoryOpenVPN,
		ConfigContent: discovery.BuildTunnelBearConfig(country.Host, 0, nil),
		Server:        country.Host,
		Port:          discovery.TunnelBearDirectPort,
		Group:         "TunnelBear",
		Country:       country.Name,
		CountryCode:   country.Code,
		Description:   "TunnelBear (signs in with your own TunnelBear account)",
		CreatedAt:     time.Now(),
	}

	id, err := a.store.AddProfile(profile)
	if err != nil {
		return "", err
	}

	if a.engine != nil {
		a.engine.AddLog("info", fmt.Sprintf("Added TunnelBear profile for %s", country.Name), "openvpn")
	}
	return id, nil
}
