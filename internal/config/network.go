package config

import "strings"

// This file is the single owner of the values that decide how traffic leaves the
// machine: the two local listener ports and the DNS resolvers handed to the
// generated core configs.
//
// The reason it exists is a disagreement that was actually in the code: the
// store's default primary resolver was `https://1.1.1.1/dns-query`, while the
// config builder's own fallback for an empty setting was the bare `1.1.1.1`
// (plain UDP). Two components, two ideas of the same setting, and which one won
// depended on whether config.json had ever been written. A fresh install, a
// hand-edited file and a normal install therefore produced different DNS
// behaviour on the same machine — the class of bug that shows up as "no internet
// and no explanation".
//
// Ports are here too. They are not configurable and cannot be: dispatcher,
// health monitor, sysproxy, the Telegram SOCKS link and every generated inbound
// name 10808/10809. What lives here is the one place that says what they are, so
// a new call site reads a name instead of copying a number.

const (
	// SocksPort is the SOCKS5 (mixed) listener every core is configured with.
	SocksPort = 10808
	// HttpPort is the HTTP listener, and the port the Windows system proxy is
	// pointed at.
	HttpPort = 10809
)

// Resolver defaults. Both are Cloudflare/Google public resolvers; the primary is
// DoH because a plain UDP query is trivially poisoned, with the caveat that some
// networks reset DoH to 1.1.1.1:443 — which is why a Layer-3 primary resolves
// through the operating system instead (see BuildSingBoxConfigAdvanced).
const (
	DefaultPrimaryResolver   = "https://1.1.1.1/dns-query"
	DefaultSecondaryResolver = "8.8.8.8"
)

// PrimaryResolver returns the resolver the generated config should use for
// everything the tunnel does not answer itself.
func (s Settings) PrimaryResolver() string {
	return normalizeResolver(s.DNSServer, DefaultPrimaryResolver)
}

// SecondaryResolver returns the second resolver: the one that bootstraps the
// tunnel endpoint, where trusting the local resolver is the risk.
func (s Settings) SecondaryResolver() string {
	return normalizeResolver(s.DNSFallback, DefaultSecondaryResolver)
}

// normalizeResolver applies the one transport rule the cores need, and keeps the
// default honest: a bare IPv4 literal is upgraded to an explicit TCP query.
//
// A bare address would be UDP/53, which is the easiest name to poison; and DoH is
// left exactly as written because rewriting `https://…` behind the user's back
// is how a resolver setting stops meaning what the UI says it means. (The older
// comment in the builder blamed DoH for `no raw query` errors on HTTPS/SVCB
// records in sing-box 1.10 — that is a reason not to *invent* DoH here, not a
// reason to remove it when it was chosen.)
func normalizeResolver(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}

	switch value {
	case "1.1.1.1":
		return "tcp://1.1.1.1"
	case "8.8.8.8":
		return "tcp://8.8.8.8"
	}
	return value
}
