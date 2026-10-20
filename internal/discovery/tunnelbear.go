package discovery

import (
	"fmt"
	"strings"
)

// TunnelBear — the operator's own published OpenVPN endpoints.
//
// TunnelBear is a commercial VPN whose OpenVPN service is documented for manual
// setup: the operator publishes one `.ovpn` per country and the user signs in
// with their OWN TunnelBear account. Nothing here is a bypass of that account —
// the username and password the user saves in the app are exactly the ones the
// operator's servers authenticate against, and a profile without them cannot
// connect.
//
// ## Why the catalogue is built rather than downloaded
//
// The published files carry no secrets and no per-user material: every one of
// them is a handful of fixed directives plus `remote <cc>.lazerpenguin.com 443`,
// and they all reference a single shared CA file by name. So a country is fully
// described by its hostname, and the whole catalogue is 47 hostnames plus one
// certificate. That means this feature needs no network access to work — which
// matters, because the networks where it is most needed are the ones where
// reaching the operator's website is hardest.
//
// ## The CA is a public certificate
//
// `TunnelBearCA` is the server-certificate authority the operator's OpenVPN
// servers present, issued to McAfee, LLC (TunnelBear's parent) and valid to
// 2034. A CA certificate is public by design — it is the material a client must
// possess in order to verify the server, and the operator distributes it for
// exactly that purpose. It is not a key, a credential, or a per-user secret.
//
// NexVPN inlines it as a `<ca>` block instead of shipping it beside the profile
// because its OpenVPN adapter writes each profile to a temporary file and runs
// `openvpn --config` on it: a relative `ca openvpn-server-ca.crt` reference
// would have to resolve against the OpenVPN binary's own directory, which is
// not ours to write into.
//
// ## The relay
//
// From Iran the operator's endpoints are reachable but the OpenVPN control
// channel is not usable as-is: UDP is dropped outright, and on TCP the TLS
// handshake inside the control channel is cut after the first few records
// because the filter recognises the fixed rhythm of OpenVPN's framing. The
// split relay (internal/protocols/ovpnrelay.go) carries the connection instead
// — see that file for what it does and why. `BuildTunnelBearConfig` takes the
// relay's port for this reason; with no relay it emits the plain UDP form,
// which is what works from an unfiltered network.

// TunnelBearSourceID is the catalogue registry key.
const TunnelBearSourceID = "tunnelbear"

// TunnelBearTCPPort is the port the operator's servers accept OpenVPN over TCP
// on. The published profiles say `proto udp` / port 443; TCP is served on a
// different port entirely, and it is the only transport worth attempting from a
// filtered network.
const TunnelBearTCPPort = 7011

// TunnelBearDirectPort is the port in the operator's own published profiles.
const TunnelBearDirectPort = 443

// TunnelBearHostSuffix is the domain the operator's OpenVPN endpoints live
// under. It is checked when deciding whether a profile is a TunnelBear one, so
// it must stay in step with the catalogue below.
const TunnelBearHostSuffix = ".lazerpenguin.com"

// TunnelBearCA is the operator's OpenVPN server CA, issued to McAfee, LLC.
//
// Kept as the raw PEM with no `<ca>` wrapper so callers can also write it to a
// file if they ever need to; the config builder adds the wrapper.
const TunnelBearCA = `-----BEGIN CERTIFICATE-----
MIICPTCCAcOgAwIBAgIQfs/kxYEHK0ojKgXA1FrgFjAKBggqhkjOPQQDAjBgMQsw
CQYDVQQGEwJDQTEUMBIGA1UECgwLTWNBZmVlLCBMTEMxDDAKBgNVBAsMA1ZQTjEt
MCsGA1UEAwwkTWNBZmVlIE9wZW5WUE4gQ2VydGlmaWNhdGUgQXV0aG9yaXR5MB4X
DTI0MDgwNjIwMzM0N1oXDTM0MDgwNjIxMzI0N1owYDELMAkGA1UEBhMCQ0ExFDAS
BgNVBAoMC01jQWZlZSwgTExDMQwwCgYDVQQLDANWUE4xLTArBgNVBAMMJE1jQWZl
ZSBPcGVuVlBOIENlcnRpZmljYXRlIEF1dGhvcml0eTB2MBAGByqGSM49AgEGBSuB
BAAiA2IABNJps+fTiqQfpGzgpq9yAPM0rLzVZ1qscVxqag3ESsclEp/uk+HCAwK1
EiLER8xXXweW9jVcYEHLuUkmBL+0FjocD5lI6zbrwaY8gWOz8vAP0fjolhXQgHfH
TqrYC9unIqNCMEAwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUwa+4wbD5vocC
xKeVNouzvsSPLekwDgYDVR0PAQH/BAQDAgGGMAoGCCqGSM49BAMCA2gAMGUCMGIM
dRbutNNzP8GIyGHKtPd+7CSOlpqeBOUBsGLkj4F1y7/yqv7hIchtTIZQymmthAIx
APY7ZiCKYW7L0mLVgowDRSY95Qxrs9NjsyQxlqRdMKcQfrojIH8Dh931M5Sj7EqR
eg==
-----END CERTIFICATE-----`

// TunnelBearCountry is one entry of the catalogue: a country name for display,
// an ISO code for the flag, and the hostname the operator publishes for it.
type TunnelBearCountry struct {
	ID string `json:"id"`
	// Name is the English label, used as the fallback and in profile names.
	Name string `json:"name"`
	// NameFA is the Persian label. It lives here rather than in the i18n table
	// because the catalogue is a fixed 47-row list owned by this package, and
	// splitting the two halves of one row across two files is how they drift.
	NameFA string `json:"name_fa"`
	// Code is the ISO 3166-1 alpha-2 code, for the flag.
	Code string `json:"code"`
	// Host is the endpoint the operator publishes for this country. One name
	// resolves to a pool of roughly twenty servers, so a "country" is really a
	// set of addresses — the relay resolves it and picks one that answers.
	Host string `json:"host"`
}

// tunnelBearCountries is the operator's published endpoint set.
//
// The United States is three entries because the operator publishes three
// distinct pools for it; they share the `us` flag but not a hostname, so the ID
// is the host prefix rather than the country code.
var tunnelBearCountries = []TunnelBearCountry{
	{ID: "ar", Name: "Argentina", NameFA: "آرژانتین", Code: "AR", Host: "ar.lazerpenguin.com"},
	{ID: "au", Name: "Australia", NameFA: "استرالیا", Code: "AU", Host: "au.lazerpenguin.com"},
	{ID: "at", Name: "Austria", NameFA: "اتریش", Code: "AT", Host: "at.lazerpenguin.com"},
	{ID: "be", Name: "Belgium", NameFA: "بلژیک", Code: "BE", Host: "be.lazerpenguin.com"},
	{ID: "br", Name: "Brazil", NameFA: "برزیل", Code: "BR", Host: "br.lazerpenguin.com"},
	{ID: "bg", Name: "Bulgaria", NameFA: "بلغارستان", Code: "BG", Host: "bg.lazerpenguin.com"},
	{ID: "ca", Name: "Canada", NameFA: "کانادا", Code: "CA", Host: "ca.lazerpenguin.com"},
	{ID: "cl", Name: "Chile", NameFA: "شیلی", Code: "CL", Host: "cl.lazerpenguin.com"},
	{ID: "co", Name: "Colombia", NameFA: "کلمبیا", Code: "CO", Host: "co.lazerpenguin.com"},
	{ID: "cy", Name: "Cyprus", NameFA: "قبرس", Code: "CY", Host: "cy.lazerpenguin.com"},
	{ID: "cz", Name: "Czech Republic", NameFA: "جمهوری چک", Code: "CZ", Host: "cz.lazerpenguin.com"},
	{ID: "dk", Name: "Denmark", NameFA: "دانمارک", Code: "DK", Host: "dk.lazerpenguin.com"},
	{ID: "fi", Name: "Finland", NameFA: "فنلاند", Code: "FI", Host: "fi.lazerpenguin.com"},
	{ID: "fr", Name: "France", NameFA: "فرانسه", Code: "FR", Host: "fr.lazerpenguin.com"},
	{ID: "de", Name: "Germany", NameFA: "آلمان", Code: "DE", Host: "de.lazerpenguin.com"},
	{ID: "gr", Name: "Greece", NameFA: "یونان", Code: "GR", Host: "gr.lazerpenguin.com"},
	{ID: "hu", Name: "Hungary", NameFA: "مجارستان", Code: "HU", Host: "hu.lazerpenguin.com"},
	{ID: "ie", Name: "Ireland", NameFA: "ایرلند", Code: "IE", Host: "ie.lazerpenguin.com"},
	{ID: "it", Name: "Italy", NameFA: "ایتالیا", Code: "IT", Host: "it.lazerpenguin.com"},
	{ID: "jp", Name: "Japan", NameFA: "ژاپن", Code: "JP", Host: "jp.lazerpenguin.com"},
	{ID: "ke", Name: "Kenya", NameFA: "کنیا", Code: "KE", Host: "ke.lazerpenguin.com"},
	{ID: "lv", Name: "Latvia", NameFA: "لتونی", Code: "LV", Host: "lv.lazerpenguin.com"},
	{ID: "lt", Name: "Lithuania", NameFA: "لیتوانی", Code: "LT", Host: "lt.lazerpenguin.com"},
	{ID: "my", Name: "Malaysia", NameFA: "مالزی", Code: "MY", Host: "my.lazerpenguin.com"},
	{ID: "mx", Name: "Mexico", NameFA: "مکزیک", Code: "MX", Host: "mx.lazerpenguin.com"},
	{ID: "md", Name: "Moldova", NameFA: "مولداوی", Code: "MD", Host: "md.lazerpenguin.com"},
	{ID: "nl", Name: "Netherlands", NameFA: "هلند", Code: "NL", Host: "nl.lazerpenguin.com"},
	{ID: "nz", Name: "New Zealand", NameFA: "نیوزیلند", Code: "NZ", Host: "nz.lazerpenguin.com"},
	{ID: "ng", Name: "Nigeria", NameFA: "نیجریه", Code: "NG", Host: "ng.lazerpenguin.com"},
	{ID: "no", Name: "Norway", NameFA: "نروژ", Code: "NO", Host: "no.lazerpenguin.com"},
	{ID: "pe", Name: "Peru", NameFA: "پرو", Code: "PE", Host: "pe.lazerpenguin.com"},
	{ID: "pl", Name: "Poland", NameFA: "لهستان", Code: "PL", Host: "pl.lazerpenguin.com"},
	{ID: "pt", Name: "Portugal", NameFA: "پرتغال", Code: "PT", Host: "pt.lazerpenguin.com"},
	{ID: "ro", Name: "Romania", NameFA: "رومانی", Code: "RO", Host: "ro.lazerpenguin.com"},
	{ID: "rs", Name: "Serbia", NameFA: "صربستان", Code: "RS", Host: "rs.lazerpenguin.com"},
	{ID: "sg", Name: "Singapore", NameFA: "سنگاپور", Code: "SG", Host: "sg.lazerpenguin.com"},
	{ID: "si", Name: "Slovenia", NameFA: "اسلوونی", Code: "SI", Host: "si.lazerpenguin.com"},
	{ID: "za", Name: "South Africa", NameFA: "آفریقای جنوبی", Code: "ZA", Host: "za.lazerpenguin.com"},
	{ID: "kr", Name: "South Korea", NameFA: "کره جنوبی", Code: "KR", Host: "kr.lazerpenguin.com"},
	{ID: "es", Name: "Spain", NameFA: "اسپانیا", Code: "ES", Host: "es.lazerpenguin.com"},
	{ID: "se", Name: "Sweden", NameFA: "سوئد", Code: "SE", Host: "se.lazerpenguin.com"},
	{ID: "ch", Name: "Switzerland", NameFA: "سوئیس", Code: "CH", Host: "ch.lazerpenguin.com"},
	{ID: "tw", Name: "Taiwan, China", NameFA: "تایوان، چین", Code: "TW", Host: "tw.lazerpenguin.com"},
	{ID: "uk", Name: "United Kingdom", NameFA: "بریتانیا", Code: "GB", Host: "uk.lazerpenguin.com"},
	{ID: "us-central", Name: "United States Central", NameFA: "مرکز آمریکا", Code: "US", Host: "us-central.lazerpenguin.com"},
	{ID: "us-east", Name: "United States East", NameFA: "شرق آمریکا", Code: "US", Host: "us-east.lazerpenguin.com"},
	{ID: "us-west", Name: "United States West", NameFA: "غرب آمریکا", Code: "US", Host: "us-west.lazerpenguin.com"},
}

// ListTunnelBearCountries returns a defensive copy of the catalogue.
func ListTunnelBearCountries() []TunnelBearCountry {
	out := make([]TunnelBearCountry, len(tunnelBearCountries))
	copy(out, tunnelBearCountries)
	return out
}

// LookupTunnelBearCountry finds one entry by ID.
func LookupTunnelBearCountry(id string) (TunnelBearCountry, bool) {
	for _, c := range tunnelBearCountries {
		if c.ID == id {
			return c, true
		}
	}
	return TunnelBearCountry{}, false
}

// IsTunnelBearHost reports whether an address belongs to the operator's OpenVPN
// endpoint set.
//
// Matching is on the domain suffix, not on the catalogue, so a profile imported
// by hand — or a hostname the operator adds later — is still recognised as
// needing the TunnelBear treatment (an account, and the relay on a filtered
// network) rather than being dialled as a plain OpenVPN server.
func IsTunnelBearHost(host string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(host)), TunnelBearHostSuffix)
}

// LookupTunnelBearCountryByHost finds the catalogue entry for an endpoint.
func LookupTunnelBearCountryByHost(host string) (TunnelBearCountry, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, c := range tunnelBearCountries {
		if strings.EqualFold(c.Host, host) {
			return c, true
		}
	}
	return TunnelBearCountry{}, false
}

// CountryForHost returns the catalogue entry for a host, or a synthetic one.
//
// A profile whose host belongs to the operator but is not in the catalogue —
// one the user imported by hand, or a country the operator added after this
// build — still has to be dialled, so it is labelled from the host itself
// rather than rejected. `de.lazerpenguin.com` becomes "DE".
func CountryForHost(host string) TunnelBearCountry {
	host = strings.ToLower(strings.TrimSpace(host))
	if c, ok := LookupTunnelBearCountryByHost(host); ok {
		return c
	}
	label := host
	if i := strings.Index(host, "."); i > 0 {
		label = host[:i]
	}
	code := strings.ToUpper(label)
	if len(code) != 2 {
		code = ""
	}
	return TunnelBearCountry{
		ID:     label,
		Name:   strings.ToUpper(label),
		NameFA: strings.ToUpper(label),
		Code:   code,
		Host:   host,
	}
}

// BuildTunnelBearConfig renders the `.ovpn` text for one endpoint.
//
// relayPort <= 0 produces the operator's own direct form (UDP/443), which is
// what works from an unfiltered network. relayPort > 0 points the profile at
// the loopback split relay and pins every relay address to the original
// gateway.
//
// The `route ... net_gateway` lines are not optional. `redirect-gateway` sends
// the whole IPv4 table down the tunnel, and the relay's own socket to the real
// server is a connection this machine originates — without the host route it is
// captured by the tunnel the relay is still trying to establish, and the
// connection deadlocks. `net_gateway` is OpenVPN's name for "via the gateway
// that was default before I started", so the route is installed by OpenVPN
// itself at the right moment and needs no external `route add`.
func BuildTunnelBearConfig(host string, relayPort int, relayIPs []string) string {
	host = strings.TrimSpace(host)
	var b strings.Builder

	b.WriteString("client\n")
	// `dev tun`, not the `dev tun0` of the published files: the adapter name is
	// NexVPN's to choose (it passes --windows-driver wintun), and a pinned
	// device name is exactly what its sanitizer strips from imported profiles.
	b.WriteString("dev tun\n")
	b.WriteString("nobind\n")
	b.WriteString("persist-key\n")
	b.WriteString("persist-tun\n")
	b.WriteString("remote-cert-tls server\n")
	b.WriteString("reneg-sec 0\n")
	b.WriteString("dhcp-option DNS 8.8.8.8\n")
	b.WriteString("dhcp-option DNS 8.8.4.4\n")
	b.WriteString("redirect-gateway\n")
	b.WriteString("verb 3\n")
	b.WriteString("auth-user-pass\n")
	// The operator names the cipher only in `data-ciphers`, which OpenVPN 2.6
	// understands but which older builds — and any tool reading the profile —
	// treat as absent. Emitting the single negotiated cipher as `cipher` too
	// costs nothing and removes that ambiguity.
	b.WriteString("data-ciphers AES-256-CBC\n")
	b.WriteString("cipher AES-256-CBC\n")
	b.WriteString("auth SHA256\n")

	if relayPort > 0 {
		// TCP, because the relay is what carries the connection and the relay
		// speaks TCP: the operator serves TCP on TunnelBearTCPPort and the
		// published profiles' UDP transport is the part that a filtered network
		// drops first. `explicit-exit-notify` is deliberately absent — it is a
		// UDP-only directive and OpenVPN refuses the whole config if it appears
		// alongside a TCP remote.
		b.WriteString("proto tcp-client\n")
		b.WriteString(fmt.Sprintf("remote 127.0.0.1 %d tcp-client\n", relayPort))
		for _, ip := range relayIPs {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("route %s 255.255.255.255 net_gateway\n", ip))
		}
	} else {
		b.WriteString("proto udp\n")
		b.WriteString(fmt.Sprintf("remote %s %d\n", host, TunnelBearDirectPort))
	}

	b.WriteString("<ca>\n")
	b.WriteString(TunnelBearCA)
	b.WriteString("\n</ca>\n")

	return b.String()
}
