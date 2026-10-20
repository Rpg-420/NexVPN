package parser

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nexvpn/internal/config"
)

func randRange(minV, maxV int) int {
	if maxV <= minV {
		return minV
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(maxV-minV+1)))
	return minV + int(n.Int64())
}

type ParsedNode struct {
	Name          string
	Protocol      config.Protocol
	Category      config.ProfileCategory
	Server        string
	Port          int
	RawURI        string
	ConfigContent string
	Group         string
	Username      string
	Password      string
	Outbound      map[string]interface{}
}

type IPInfo struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
}

func ParseURI(rawURI string) (*ParsedNode, error) {
	rawURI = strings.TrimSpace(rawURI)
	if strings.HasPrefix(rawURI, "vless://") {
		return parseVless(rawURI)
	}
	if strings.HasPrefix(rawURI, "vmess://") {
		return parseVmess(rawURI)
	}
	if strings.HasPrefix(rawURI, "trojan://") {
		return parseTrojan(rawURI)
	}
	if strings.HasPrefix(rawURI, "ss://") {
		return parseShadowsocks(rawURI)
	}
	if strings.HasPrefix(rawURI, "hy2://") || strings.HasPrefix(rawURI, "hysteria2://") {
		return parseHysteria2(rawURI)
	}
	if strings.HasPrefix(rawURI, "tuic://") {
		return parseTuic(rawURI)
	}
	if strings.HasPrefix(rawURI, "wireguard://") || strings.HasPrefix(rawURI, "wg://") {
		return parseWireguardURI(rawURI)
	}
	return nil, fmt.Errorf("unsupported URI scheme: %s", rawURI)
}

// BuildNodeConfigWithOptions dynamically builds the optimal configuration with full settings support
func BuildNodeConfigWithOptions(profile *config.Profile, settings config.Settings) (config.Protocol, string, error) {
	proxyMode := settings.ProxyMode
	if proxyMode == "" {
		proxyMode = "tun"
	}
	routingMode := settings.RoutingMode
	if routingMode == "" {
		routingMode = "bypass_iran"
	}

	// 1. OpenVPN profile
	if profile.Protocol == config.ProtocolOpenVPN || profile.Category == config.CategoryOpenVPN {
		return config.ProtocolOpenVPN, profile.ConfigContent, nil
	}

	// 2. WireGuard .conf content
	if profile.Protocol == config.ProtocolWireGuard && profile.RawURI == "" && strings.Contains(profile.ConfigContent, "[Interface]") {
		outbound, host, err := ParseWireGuardConf(profile.ConfigContent)
		if err == nil && outbound != nil {
			sbJSON, err := BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, host, settings)
			if err == nil {
				return config.ProtocolSingBox, sbJSON, nil
			}
		}
	}

	// 3. Profiles with RawURI (VLESS, VMess, Trojan, SS, Hy2, TUIC, WireGuard, WARP)
	if profile.RawURI != "" {
		uri := strings.TrimSpace(profile.RawURI)
		isUDP := strings.HasPrefix(uri, "hy2://") || strings.HasPrefix(uri, "hysteria2://") || strings.HasPrefix(uri, "tuic://") || strings.HasPrefix(uri, "wireguard://") || strings.HasPrefix(uri, "wg://")

		if (profile.Protocol == config.ProtocolXray || (proxyMode == "system_proxy" && !isUDP)) && !isUDP {
			xrayJSON, err := BuildXrayConfigFromURI(uri, 10809, 10808, routingMode, settings)
			if err == nil && xrayJSON != "" {
				return config.ProtocolXray, xrayJSON, nil
			}
		}

		outbound, err := ParseOutboundFromURI(profile.RawURI)
		if err == nil && outbound != nil {
			host := profile.Server
			if host == "" {
				if s, ok := outbound["server"].(string); ok {
					host = s
				}
			}

			// In TUN mode or UDP-based protocols (hy2, tuic, wireguard, warp), use Sing-Box
			sbJSON, err := BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, host, settings)
			if err == nil {
				return config.ProtocolSingBox, sbJSON, nil
			}
		}
	}

	// 4. Raw JSON configs (Sing-box or Xray)
	if profile.ConfigContent != "" {
		trimmed := strings.TrimSpace(profile.ConfigContent)
		if strings.HasPrefix(trimmed, "{") {
			var testMap map[string]interface{}
			if err := json.Unmarshal([]byte(trimmed), &testMap); err == nil {
				// Detect if it is an Xray config
				if _, hasInbounds := testMap["inbounds"]; hasInbounds {
					if _, hasOutbounds := testMap["outbounds"]; hasOutbounds {
						if _, hasRouting := testMap["routing"]; hasRouting {
							return config.ProtocolXray, profile.ConfigContent, nil
						}
					}
				}
				// Default JSON is treated as Sing-Box
				return config.ProtocolSingBox, profile.ConfigContent, nil
			}
		}
		return profile.Protocol, profile.ConfigContent, nil
	}

	return profile.Protocol, "", fmt.Errorf("profile has no valid configuration or URI")
}

// BuildNodeConfig dynamically builds the optimal configuration and selects the best core for any profile
func BuildNodeConfig(profile *config.Profile, proxyMode string, routingMode string) (config.Protocol, string, error) {
	settings := config.Settings{
		ProxyMode:   proxyMode,
		RoutingMode: routingMode,
	}
	return BuildNodeConfigWithOptions(profile, settings)
}

// dnsQueryTimeout bounds a single DNS exchange inside the core.
//
// sing-box waits 10s per exchange by default. When a resolver is unreachable
// that cost is paid *per connection*, so one dead DNS server turned into ~90
// `context deadline exceeded` lines spread over two minutes in the 2026-10-11
// log while the UI only looked sluggish. Three seconds is longer than any
// healthy exchange on a desktop link and short enough that a dead one fails
// visibly instead of hanging the whole tunnel.
//
// It is a `dns` block option, not a per-server one: sing-box 1.14 rejects
// `dns.servers[].timeout` with `unknown field`, and the generated config is
// checked against the bundled core by the parser tests for exactly that reason.
const dnsQueryTimeout = "3s"

// makeSingBoxDNSServer parses a DNS address and returns a Sing-box 1.14 compliant DNS server configuration
func makeSingBoxDNSServer(tag, rawAddr, detour string) map[string]interface{} {
	srv := map[string]interface{}{
		"tag": tag,
	}
	if detour != "" {
		srv["detour"] = detour
	}

	if rawAddr == "local" {
		srv["type"] = "local"
		return srv
	}

	u, err := url.Parse(rawAddr)
	if err != nil || u.Scheme == "" {
		host := rawAddr
		port := 53
		if h, p, err := net.SplitHostPort(rawAddr); err == nil {
			host = h
			if pInt, err := strconv.Atoi(p); err == nil {
				port = pInt
			}
		}
		srv["type"] = "udp"
		srv["server"] = host
		srv["server_port"] = port
		return srv
	}

	switch strings.ToLower(u.Scheme) {
	case "tcp":
		port := 53
		host := u.Host
		if h, p, err := net.SplitHostPort(u.Host); err == nil {
			host = h
			if pInt, err := strconv.Atoi(p); err == nil {
				port = pInt
			}
		}
		srv["type"] = "tcp"
		srv["server"] = host
		srv["server_port"] = port
	case "tls":
		port := 853
		host := u.Host
		if h, p, err := net.SplitHostPort(u.Host); err == nil {
			host = h
			if pInt, err := strconv.Atoi(p); err == nil {
				port = pInt
			}
		}
		srv["type"] = "tls"
		srv["server"] = host
		srv["server_port"] = port
	case "https":
		port := 443
		host := u.Host
		if h, p, err := net.SplitHostPort(u.Host); err == nil {
			host = h
			if pInt, err := strconv.Atoi(p); err == nil {
				port = pInt
			}
		}
		srv["type"] = "https"
		srv["server"] = host
		srv["server_port"] = port
		path := u.Path
		if path == "" {
			path = "/dns-query"
		}
		srv["path"] = path
	case "udp":
		fallthrough
	default:
		port := 53
		host := u.Host
		if host == "" {
			host = u.Opaque
		}
		if h, p, err := net.SplitHostPort(host); err == nil {
			host = h
			if pInt, err := strconv.Atoi(p); err == nil {
				port = pInt
			}
		}
		srv["type"] = "udp"
		srv["server"] = host
		srv["server_port"] = port
	}

	return srv
}

func toSafeStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		var res []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				res = append(res, s)
			}
		}
		return res
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func toSafeInt(val interface{}, defaultVal int) int {
	if val == nil {
		return defaultVal
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func toSafeIntSlice(val interface{}) []int {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []int:
		return v
	case []interface{}:
		var res []int
		for _, item := range v {
			switch num := item.(type) {
			case float64:
				res = append(res, int(num))
			case int:
				res = append(res, num)
			case int64:
				res = append(res, int(num))
			}
		}
		return res
	}
	return nil
}

// convertWireGuardOutboundToEndpoint converts a WireGuard outbound map to a Sing-box 1.13+ endpoint map
func convertWireGuardOutboundToEndpoint(ob map[string]interface{}) map[string]interface{} {
	tag := "proxy"
	if t, ok := ob["tag"].(string); ok && t != "" {
		tag = t
	}
	addrs := toSafeStringSlice(ob["address"])
	if len(addrs) == 0 {
		addrs = toSafeStringSlice(ob["local_address"])
	}
	if len(addrs) == 0 {
		// Fallback to standard Cloudflare WARP client address if missing
		addrs = []string{"172.16.0.2/32", "2606:4700:110:8cc2:617d:9326:86fe:3b6e/128"}
	}
	privKey, _ := ob["private_key"].(string)
	peerPubKey, _ := ob["peer_public_key"].(string)
	server, _ := ob["server"].(string)
	serverPort := toSafeInt(ob["server_port"], 51820)
	peer := map[string]interface{}{
		"address":     server,
		"port":        serverPort,
		"public_key":  peerPubKey,
		"allowed_ips": []string{"0.0.0.0/0", "::/0"},
	}
	res := toSafeIntSlice(ob["reserved"])
	if len(res) > 0 {
		peer["reserved"] = res
	}
	ep := map[string]interface{}{
		"type":        "wireguard",
		"tag":         tag,
		"address":     addrs,
		"private_key": privKey,
		"peers":       []map[string]interface{}{peer},
	}
	mtu := toSafeInt(ob["mtu"], 1280)
	if mtu > 0 {
		ep["mtu"] = mtu
	}
	if detour, ok := ob["detour"].(string); ok && detour != "" {
		ep["detour"] = detour
	}
	return ep
}

// BuildSingBoxConfig constructs standard Sing-box configuration
func BuildSingBoxConfig(proxyOutbound map[string]interface{}, proxyMode string, routingMode string, serverHost string) (string, error) {
	return BuildSingBoxConfigAdvanced(proxyOutbound, proxyMode, routingMode, serverHost, config.Settings{
		ProxyMode:   proxyMode,
		RoutingMode: routingMode,
	})
}

// BuildSingBoxConfigAdvanced constructs a state-of-the-art Sing-box 1.9+ config with Anti-DPI, App Split Tunneling, and Multi-Hop
// tunIfaceName returns the Wintun adapter name to write into a TUN inbound.
//
// It is normally the default, but the engine can override it per connection via
// Settings.TunInterfaceName. That override exists for one situation: a *ghost*
// adapter occupying the default name. The Wintun driver still answers to a
// ghost's name, so the core sees the name as taken, opens a handle to nothing,
// and exits during startup — while the OS adapter list cannot see it, which
// means nothing can delete it either. Renaming around it is the only remedy that
// does not require registry surgery.
func tunIfaceName(settings config.Settings) string {
	return settings.EffectiveTunInterfaceName()
}

func BuildSingBoxConfigAdvanced(proxyOutbound map[string]interface{}, proxyMode string, routingMode string, serverHost string, settings config.Settings) (string, error) {
	// Deep copy outbound so mutations don't affect parent
	outboundBytes, _ := json.Marshal(proxyOutbound)
	var activeOutbound map[string]interface{}
	_ = json.Unmarshal(outboundBytes, &activeOutbound)

	// WireGuard outbound cleanup: remove fields unsupported by sing-box stable
	// AmneziaWG fields (junk_packet_*, s1/s2, h1-4) only supported in sing-box >=1.11 / Amnezia fork
	// To avoid FATAL on sing-box 1.10.7 (still bundled), strip them unless explicitly needed
	if outType, ok := activeOutbound["type"].(string); ok && outType == "wireguard" {
		delete(activeOutbound, "tls")
		delete(activeOutbound, "tls_tricks")
		// Strip Amnezia obfuscation for compatibility with sing-box 1.10.x stable
		// Will be re-enabled automatically when sing-box >=1.11 is detected at runtime
		delete(activeOutbound, "junk_packet_count")
		delete(activeOutbound, "junk_packet_min_size")
		delete(activeOutbound, "junk_packet_max_size")
		delete(activeOutbound, "init_packet_junk_size")
		delete(activeOutbound, "response_packet_junk_size")
		delete(activeOutbound, "init_packet_magic_header")
		delete(activeOutbound, "response_packet_magic_header")
		delete(activeOutbound, "underload_packet_magic_header")
		delete(activeOutbound, "transport_header_magic_header")
	}

	// Inject TLS ClientHello fragmentation anti-censorship parameters if enabled
	if settings.FragmentEnabled {
		if tlsMap, ok := activeOutbound["tls"].(map[string]interface{}); ok {
			// Fragmentation must not be injected on Reality TLS: the Reality
			// handshake relies on an unmodified ClientHello.
			realityObj, hasReality := tlsMap["reality"].(map[string]interface{})
			isReality := hasReality && realityObj != nil && realityObj["enabled"] == true
			if !isReality {
				// sing-box 1.12+ takes `tls.fragment` as a **bool**. The older
				// {enabled,size,sleep} object was removed in 1.12, and feeding it
				// to the bundled 1.14.0 core makes it exit during startup with
				// `outbounds[N].tls.fragment: cannot unmarshal object ... of type
				// bool` — a FATAL config-decode failure, so the connection never
				// even reaches the TUN stage.
				//
				// `record_fragment` splits the handshake across several TLS
				// records rather than several packets; the two are complementary
				// and both are on for the same reason FragmentEnabled exists.
				//
				// FragmentSize / FragmentSleep cannot be expressed in the sing-box
				// schema at all, so they only affect the Xray path (see the
				// `dialerProxy` fragment outbound further down this file).
				tlsMap["fragment"] = true
				tlsMap["record_fragment"] = true
			}
		}
	}

	listenAddr := "127.0.0.1"
	if settings.AllowLan {
		listenAddr = "0.0.0.0"
	}

	// Inbounds: Mixed on the SOCKS port (SOCKS5/HTTP) AND a dedicated HTTP
	// listener, because the Windows system proxy cannot speak SOCKS. Both numbers
	// come from config, the single owner (internal/config/network.go).
	inbounds := []map[string]interface{}{
		{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      listenAddr,
			"listen_port": config.SocksPort,
		},
		{
			"type":        "http",
			"tag":         "http-in",
			"listen":      listenAddr,
			"listen_port": config.HttpPort,
		},
	}

	// Add High-Performance TUN Inbound for full OS traffic redirection
	if proxyMode == "tun" || proxyMode == "" {
		tunStack := "mixed"
		tunMTU := 1500
		if settings.GameModeEnabled {
			tunMTU = 1400
		}

		tunInbound := map[string]interface{}{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": tunIfaceName(settings),
			"address": []string{
				"172.19.0.1/30",
				"fdfe:dcba:9876::1/126",
			},
			"auto_route":   true,
			"strict_route": false,
			"stack":        tunStack,
			"mtu":          tunMTU,
		}
		inbounds = append(inbounds, tunInbound)
	}

	primaryTag := "proxy"
	var outbounds []map[string]interface{}
	var endpoints []map[string]interface{}

	// Always add direct and block outbounds first to ensure they exist for DNS detour
	outbounds = append(outbounds,
		map[string]interface{}{
			"type": "direct",
			"tag":  "direct",
		},
		map[string]interface{}{
			"type": "block",
			"tag":  "block",
		},
	)

	if outType, ok := activeOutbound["type"].(string); ok && outType == "wireguard" {
		// WireGuard outbound cleanup: remove fields unsupported by sing-box stable
		// AmneziaWG fields (junk_packet_*, s1/s2, h1-4) only supported in sing-box >=1.11 / Amnezia fork
		// To avoid FATAL on sing-box 1.10.7 (still bundled), strip them unless explicitly needed
		if outType, ok := activeOutbound["type"].(string); ok && outType == "wireguard" {
			delete(activeOutbound, "tls")
			delete(activeOutbound, "tls_tricks")
			// Strip Amnezia obfuscation for compatibility with sing-box 1.10.x stable
			// Will be re-enabled automatically when sing-box >=1.11 is detected at runtime
			delete(activeOutbound, "junk_packet_count")
			delete(activeOutbound, "junk_packet_min_size")
			delete(activeOutbound, "junk_packet_max_size")
			delete(activeOutbound, "init_packet_junk_size")
			delete(activeOutbound, "response_packet_junk_size")
			delete(activeOutbound, "init_packet_magic_header")
			delete(activeOutbound, "response_packet_magic_header")
			delete(activeOutbound, "underload_packet_magic_header")
			delete(activeOutbound, "transport_header_magic_header")
		}
		endpoints = append(endpoints, convertWireGuardOutboundToEndpoint(activeOutbound))
	} else {
		outbounds = append(outbounds, activeOutbound)
	}

	// Multi-Hop WARP Chain: local -> VPS proxy -> WARP -> target
	if settings.MultiHopWarp {
		warpCleanIP := "162.159.192.1"
		if settings.WarpCleanIP != "" {
			warpCleanIP = settings.WarpCleanIP
		}
		warpPort := 2408
		if settings.WarpPort > 0 {
			warpPort = settings.WarpPort
		}

		// Try to generate a fresh WARP profile for the second hop
		var warpPrivKey, warpPubKey, warpLocalAddr string
		var warpReserved []int
		if warpProfile, err := GenerateCloudflareWarp(warpCleanIP, warpPort); err == nil && warpProfile != nil {
			// Extract keys from the RawURI
			if warpProfile.RawURI != "" {
				if warpOut, parseErr := parseWireguardOutbound(warpProfile.RawURI); parseErr == nil {
					warpPrivKey, _ = warpOut["private_key"].(string)
					warpPubKey, _ = warpOut["peer_public_key"].(string)
					if addrs, ok := warpOut["local_address"].([]string); ok && len(addrs) > 0 {
						warpLocalAddr = addrs[0]
					}
					if res, ok := warpOut["reserved"].([]int); ok {
						warpReserved = res
					}
				}
			}
		}
		// Fallback to well-known WARP public key if API call failed (private key will be random)
		if warpPrivKey == "" {
			curve := ecdh.X25519()
			if priv, err := curve.GenerateKey(rand.Reader); err == nil {
				warpPrivKey = base64.StdEncoding.EncodeToString(priv.Bytes())
			}
			warpPubKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
			warpLocalAddr = "172.16.0.2/32"
		}

		warpPeer := map[string]interface{}{
			"address":     warpCleanIP,
			"port":        warpPort,
			"public_key":  warpPubKey,
			"allowed_ips": []string{"0.0.0.0/0", "::/0"},
		}
		if len(warpReserved) > 0 {
			warpPeer["reserved"] = warpReserved
		}
		warpEP := map[string]interface{}{
			"type":        "wireguard",
			"tag":         "warp-chain",
			"address":     []string{warpLocalAddr},
			"private_key": warpPrivKey,
			"mtu":         1280,
			"detour":      "proxy",
			"peers":       []map[string]interface{}{warpPeer},
		}
		endpoints = append(endpoints, warpEP)
		primaryTag = "warp-chain"
	}

	// NOTE: the "direct" and "block" outbounds are already appended above, before
	// the proxy outbound, so that the DNS detours can always resolve. Appending
	// them a second time here produced two outbounds with the same tag, which
	// sing-box rejects with "duplicate outbound/endpoint tag: direct".

	// Both resolvers come from their single owner (internal/config/network.go).
	// Before that existed, the store defaulted the primary to DoH while this
	// function defaulted it to a bare `1.1.1.1` — so whether a machine used DoH
	// or plain UDP depended on whether config.json had ever been written.
	primaryDNSAddr := settings.PrimaryResolver()
	fallbackDNSAddr := settings.SecondaryResolver()

	// A Layer-3 primary (WireGuard) moves packets, not names: the core has to know
	// the real address *before* anything can enter the tunnel, so DNS sits on the
	// critical path of every single connection. Detouring that lookup through the
	// tunnel which depends on it is a deadlock — in the 2026-10-11 log a WireGuard
	// engine answered `dns: lookup failed … context deadline exceeded` for every
	// domain it was asked about, because the tunnel it was querying through was
	// the tunnel that was not carrying traffic.
	//
	// Resolving straight out is the trade: query names become visible to the
	// local network, exactly as they do with a standalone WireGuard client, and it
	// is the only variant that works whether or not the peer forwards DNS at all.
	//
	// The way to express that is to *omit* the detour, not to point it at
	// `direct`. Since sing-box 1.12 a DNS server builds its own dialer, and a
	// server with no detour "is equivalent to using an empty direct outbound by
	// default" — it dials straight out. Naming the bare `direct` outbound
	// explicitly instead trips a startup FATAL:
	//   start dns/https[dns-remote]: detour to an empty direct outbound makes no sense
	// (sing-box #2778 / #2803). That is exactly what killed every WireGuard-engine
	// connection in the 2026-10-12 log — TUN and the System-Proxy fallback alike
	// — before the tunnel ever came up, on a config the bundled core's `check`
	// still accepts, because the rejection only happens when the service starts.
	//
	// Every other primary (VLESS / VMess / Trojan / SS / Hysteria2 / TUIC) is
	// handed the domain and resolves it remotely, so those keep the tunnel detour
	// and leak nothing.
	dnsDetour := primaryTag
	layer3Primary := outboundIsLayer3(activeOutbound)
	if layer3Primary {
		dnsDetour = ""
	}

	// Which resolver answers everything fakeip does not.
	//
	// A proxy primary sends it through the tunnel: the domain leaves with the
	// connection anyway, so nothing extra leaks.
	//
	// A Layer-3 primary gets the machine's own resolver instead, and the reason is
	// measured. In the 2026-10-12 log every single lookup of a WireGuard session
	// died with `read tcp 192.168.1.103:xxxxx->1.1.1.1:443: wsarecv: An existing
	// connection was forcibly closed` — the configured DoH server being reset by
	// the network, ~200 times, while the same machine resolves names through its
	// router (192.168.1.1) with no tunnel up at all. sing-box 1.14 has **no
	// fallback chain**: both `dns.final` and `dns.rules[].server` reject a list
	// (`cannot unmarshal array into ... of type string`, verified against the
	// bundled core), so the app has to pick the one resolver that survives this
	// network. For a bootstrap path that is the OS one.
	resolverForUnmatched := "dns-remote"
	if layer3Primary {
		resolverForUnmatched = "dns-direct"
	}

	// Which resolver finds the tunnel endpoint itself. For a Layer-3 primary that
	// is deliberately *not* the router: a poisoned local resolver is what makes a
	// clean-IP or CDN-fronted endpoint resolve to a blocked address, so the
	// bootstrap uses the user's independent second resolver, straight out. It is
	// also the only thing that gives `dns-fallback` a job — before this it was
	// written into every generated config and referenced by nothing.
	bootstrapResolver := "dns-direct"
	if layer3Primary {
		bootstrapResolver = "dns-fallback"
	}

	dnsServers := []map[string]interface{}{
		makeSingBoxDNSServer("dns-remote", primaryDNSAddr, dnsDetour),
		{
			"tag":    "dns-direct",
			"type":   "local",
			"detour": "direct",
		},
		{
			"tag":         "dns-fakeip",
			"type":        "fakeip",
			"inet4_range": "198.18.0.0/15",
			"inet6_range": "fc00::/18",
		},
		makeSingBoxDNSServer("dns-fallback", fallbackDNSAddr, dnsDetour),
	}

	// DNS Rules — FakeIP for the TUN inbound, the tunnel endpoint and domestic
	// domains to a tunnel-independent resolver; everything else follows
	// `dns.final` (resolverForUnmatched above).
	dnsRules := []map[string]interface{}{
		{
			// fakeip answers A and AAAA only. Without the query_type filter every
			// other record type arriving on the TUN inbound (HTTPS/SVCB, PTR,
			// TXT, …) is routed here as well, and the core rejects each one with
			// "only IP queries are supported by fakeip" — which floods the log
			// and leaves those lookups unanswered. Anything that is not A/AAAA
			// now falls through to the final resolver.
			"inbound":    []string{"tun-in"},
			"query_type": []string{"A", "AAAA"},
			"server":     "dns-fakeip",
		},
	}

	if serverHost != "" && !isIPAddress(serverHost) {
		dnsRules = append(dnsRules, map[string]interface{}{
			"domain": []string{serverHost},
			"server": bootstrapResolver,
		})
	}

	if routingMode == "bypass_iran" || routingMode == "direct_iran_block_ads" || routingMode == "" {
		dnsRules = append(dnsRules,
			map[string]interface{}{
				"domain_suffix": []string{".ir"},
				"server":        "dns-direct",
			},
			map[string]interface{}{
				"domain_keyword": []string{"telewebion", "digikala", "snapp", "shaparak", "divar", "torob", "aparat", "varzesh3", "bank", "bmi.ir", "sep.ir", "shaparak.ir", "irib"},
				"server":         "dns-direct",
			},
		)
	}

	if settings.BlockAds || routingMode == "direct_iran_block_ads" {
		dnsRules = append(dnsRules, map[string]interface{}{
			"domain_keyword": []string{"doubleclick", "adservice", "googleadservices", "adnxs", "adroll", "analytics", "applovin", "unityads", "vungle", "admob"},
			"action":         "reject",
		})
	}

	// Routing Rules
	lanCIDRs := []string{"192.168.0.0/16", "127.0.0.0/8", "169.254.0.0/16"}
	if settings.BypassLAN && activeOutbound["type"] != "wireguard" {
		lanCIDRs = append(lanCIDRs, "10.0.0.0/8", "172.16.0.0/12")
	}

	routeRules := []map[string]interface{}{
		{
			"action": "sniff",
		},
		{
			"network": "udp",
			"port":    []int{53},
			"action":  "hijack-dns",
		},
		{
			"protocol": []string{"dns"},
			"action":   "hijack-dns",
		},
		{
			"ip_cidr":  lanCIDRs,
			"outbound": "direct",
		},
	}

	if serverHost != "" {
		if isIPAddress(serverHost) {
			routeRules = append(routeRules, map[string]interface{}{
				"ip_cidr":  []string{serverHost + "/32"},
				"outbound": "direct",
			})
		} else {
			routeRules = append(routeRules, map[string]interface{}{
				"domain":   []string{serverHost},
				"outbound": "direct",
			})
		}
	}

	// Per-App Process Routing (Split Tunneling) - Normalized for Cross-Platform compatibility
	var processedApps []string
	if len(settings.SelectedApps) > 0 {
		for _, app := range settings.SelectedApps {
			if runtime.GOOS != "windows" {
				processedApps = append(processedApps, strings.TrimSuffix(strings.ToLower(app), ".exe"))
			} else {
				processedApps = append(processedApps, app)
			}
		}
	}

	if settings.AppRoutingMode == "bypass_selected" && len(processedApps) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"process_name": processedApps,
			"outbound":     "direct",
		})
	} else if settings.AppRoutingMode == "proxy_selected" && len(processedApps) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"process_name": processedApps,
			"outbound":     primaryTag,
		})
		// Other apps go direct
		routeRules = append(routeRules, map[string]interface{}{
			"outbound": "direct",
		})
	}

	if routingMode == "bypass_iran" || routingMode == "direct_iran_block_ads" || routingMode == "" {
		routeRules = append(routeRules,
			map[string]interface{}{
				"domain_suffix": []string{".ir"},
				"outbound":      "direct",
			},
			map[string]interface{}{
				"domain_keyword": []string{"telewebion", "digikala", "snapp", "shaparak", "divar", "torob", "aparat", "varzesh3", "bank", "bmi.ir", "sep.ir", "shaparak.ir", "irib"},
				"outbound":       "direct",
			},
		)
	}

	// Ad-blocking, Malware, and Anti-Phishing rules
	if settings.BlockAds || routingMode == "direct_iran_block_ads" {
		routeRules = append(routeRules, map[string]interface{}{
			"domain_keyword": []string{"doubleclick", "adservice", "googleadservices", "adnxs", "adroll", "analytics", "applovin", "unityads", "vungle", "admob", "tracker", "telemetry", "phishing", "malware", "spyware"},
			"outbound":       "block",
		})
	}

	// User-defined routing overrides.
	//
	// Appended after every built-in rule so that a manual entry always wins,
	// and before the IPv6 reject below so that an explicit IPv6 rule from the
	// user is honoured rather than dropped. First match wins in sing-box, so
	// this ordering is the whole contract.
	routeRules = append(routeRules, UserRoutingRulesSingBox(settings.RoutingRules, primaryTag)...)

	// Block unroutable IPv6 traffic when IPv4-only to eliminate no route to host errors
	if settings.IPStrategy != "prefer_ipv6" && settings.IPStrategy != "ipv6_only" {
		routeRules = append(routeRules, map[string]interface{}{
			"ip_cidr": []string{"::/0"},
			"action":  "reject",
			"method":  "drop",
		})
	}

	ipStrategy := "prefer_ipv4"
	if settings.IPStrategy != "" {
		ipStrategy = settings.IPStrategy
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "warn",
		},
		"experimental": map[string]interface{}{
			"clash_api": map[string]interface{}{
				"external_controller": "127.0.0.1:9090",
			},
		},
		"dns": map[string]interface{}{
			"servers":  dnsServers,
			"rules":    dnsRules,
			"final":    resolverForUnmatched,
			"strategy": ipStrategy,
			// Bound every exchange, so a resolver that is not answering costs 3s
			// per lookup instead of sing-box's 10s default. See dnsQueryTimeout.
			"timeout": dnsQueryTimeout,
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route": map[string]interface{}{
			"rules": routeRules,
			"final": primaryTag,
			// The same resolver the DNS router falls through to. Naming a
			// different one here would be the kind of split-brain the 2026-10-12
			// log shows: `dns.final` pointed at dns-remote, so the dial-time
			// lookups that decide whether a connection can exist at all went out
			// over a DoH endpoint the network was resetting, even though
			// `default_domain_resolver` said dns-direct.
			"auto_detect_interface":   true,
			"default_domain_resolver": resolverForUnmatched,
		},
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}

	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ParseOutboundFromURI converts any supported URI to a native Sing-box outbound object
func ParseOutboundFromURI(raw string) (map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "vless://") {
		return parseVlessOutbound(raw)
	}
	if strings.HasPrefix(raw, "vmess://") {
		return parseVmessOutbound(raw)
	}
	if strings.HasPrefix(raw, "trojan://") {
		return parseTrojanOutbound(raw)
	}
	if strings.HasPrefix(raw, "ss://") {
		return parseShadowsocksOutbound(raw)
	}
	if strings.HasPrefix(raw, "hy2://") || strings.HasPrefix(raw, "hysteria2://") {
		return parseHysteria2Outbound(raw)
	}
	if strings.HasPrefix(raw, "tuic://") {
		return parseTuicOutbound(raw)
	}
	if strings.HasPrefix(raw, "wireguard://") || strings.HasPrefix(raw, "wg://") {
		return parseWireguardOutbound(raw)
	}
	return nil, fmt.Errorf("unsupported URI")
}

func parseVlessOutbound(raw string) (map[string]interface{}, error) {
	// Strip URL fragment before parsing to prevent emoji/unicode errors in url.Parse
	if idx := strings.Index(raw, "#"); idx != -1 {
		raw = raw[:idx]
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	q := u.Query()
	flow := q.Get("flow")
	security := q.Get("security")
	sni := q.Get("sni")
	if sni == "" {
		hostHeader := q.Get("host")
		if hostHeader != "" && !isIPAddress(hostHeader) {
			sni = hostHeader
		}
		if sni == "" && !isIPAddress(host) {
			sni = host
		}
		// No host fallback here: the block below derives the Host header from
		// the resolved sni, so duplicating it would just be a second guess at
		// the same value.
	}
	fp := q.Get("fp")
	if fp == "" {
		fp = "chrome"
	}
	pbk := q.Get("pbk")
	sid := q.Get("sid")
	netType := q.Get("type")
	if netType == "" {
		netType = "tcp"
	}
	path := q.Get("path")
	if path != "" {
		if decodedPath, err := url.QueryUnescape(path); err == nil && decodedPath != "" {
			path = decodedPath
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	} else {
		path = "/"
	}

	hostHeader := q.Get("host")
	if hostHeader == "" {
		hostHeader = sni
	}
	if hostHeader == "" {
		hostHeader = host
	}

	outbound := map[string]interface{}{
		"type":        "vless",
		"tag":         "proxy",
		"server":      host,
		"server_port": port,
		"uuid":        uuid,
	}

	if flow != "" {
		outbound["flow"] = flow
	}

	tlsObj := map[string]interface{}{
		"enabled":     security == "tls" || security == "reality",
		"server_name": sni,
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": fp,
		},
	}

	if security == "reality" {
		cleanPbk := strings.TrimRight(pbk, "=")
		tlsObj["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": cleanPbk,
			"short_id":   sid,
		}
	}

	if security == "tls" || security == "reality" {
		outbound["tls"] = tlsObj
	}

	if netType == "ws" {
		outbound["transport"] = map[string]interface{}{
			"type": "ws",
			"path": path,
			"headers": map[string]string{
				"Host": hostHeader,
			},
		}
	} else if netType == "grpc" {
		serviceName := q.Get("serviceName")
		if serviceName == "" {
			serviceName = q.Get("service_name")
		}
		outbound["transport"] = map[string]interface{}{
			"type":         "grpc",
			"service_name": serviceName,
		}
	} else if netType == "httpupgrade" || netType == "xhttp" || netType == "splithttp" {
		// `host` already sets the Host header for the httpupgrade transport, so
		// adding a `headers: {Host: ...}` entry on top of it duplicates the same
		// value. The trojan branch uses `host` alone; keep the two consistent.
		outbound["transport"] = map[string]interface{}{
			"type": "httpupgrade",
			"path": path,
			"host": hostHeader,
		}
	} else if netType == "http" || (netType == "tcp" && q.Get("headerType") == "http") {
		outbound["transport"] = map[string]interface{}{
			"type": "http",
			"path": path,
			"host": []string{hostHeader},
		}
	}

	return outbound, nil
}

func parseVmessOutbound(raw string) (map[string]interface{}, error) {
	b64 := strings.TrimPrefix(raw, "vmess://")
	decoded, err := decodeBase64(b64)
	if err != nil {
		return nil, err
	}

	var data struct {
		Add  string      `json:"add"`
		Port interface{} `json:"port"`
		ID   string      `json:"id"`
		Aid  interface{} `json:"aid"`
		Scy  string      `json:"scy"`
		Net  string      `json:"net"`
		Host string      `json:"host"`
		Path string      `json:"path"`
		TLS  string      `json:"tls"`
		SNI  string      `json:"sni"`
	}

	if err := json.Unmarshal([]byte(decoded), &data); err != nil {
		return nil, err
	}

	port := 443
	switch v := data.Port.(type) {
	case float64:
		port = int(v)
	case string:
		port, _ = strconv.Atoi(v)
	}

	security := data.Scy
	if security == "" {
		security = "auto"
	}

	outbound := map[string]interface{}{
		"type":        "vmess",
		"tag":         "proxy",
		"server":      data.Add,
		"server_port": port,
		"uuid":        data.ID,
		"security":    security,
	}

	if data.TLS == "tls" {
		sni := data.SNI
		if sni == "" && data.Host != "" && !isIPAddress(data.Host) {
			sni = data.Host
		}
		if sni == "" && !isIPAddress(data.Add) {
			sni = data.Add
		}
		outbound["tls"] = map[string]interface{}{
			"enabled":     true,
			"server_name": sni,
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			},
		}
	}

	if data.Net == "ws" {
		hostHeader := data.Host
		if hostHeader == "" {
			hostHeader = data.Add
		}
		outbound["transport"] = map[string]interface{}{
			"type": "ws",
			"path": data.Path,
			"headers": map[string]string{
				"Host": hostHeader,
			},
		}
	} else if data.Net == "grpc" {
		outbound["transport"] = map[string]interface{}{
			"type":         "grpc",
			"service_name": data.Path,
		}
	}

	return outbound, nil
}

func parseTrojanOutbound(raw string) (map[string]interface{}, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	password := u.User.Username()
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		hostHeader := q.Get("host")
		if hostHeader != "" && !isIPAddress(hostHeader) {
			sni = hostHeader
		}
		if sni == "" && !isIPAddress(host) {
			sni = host
		}
		// No host fallback here either — see the vless branch above; the
		// Host header is derived once, further down.
	}

	outbound := map[string]interface{}{
		"type":        "trojan",
		"tag":         "proxy",
		"server":      host,
		"server_port": port,
		"password":    password,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": sni,
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			},
		},
	}

	netType := q.Get("type")
	path := q.Get("path")
	if path == "" {
		path = "/"
	}
	hostHeader := q.Get("host")
	if hostHeader == "" {
		hostHeader = sni
	}

	if netType == "ws" {
		outbound["transport"] = map[string]interface{}{
			"type": "ws",
			"path": path,
			"headers": map[string]string{
				"Host": hostHeader,
			},
		}
	} else if netType == "grpc" {
		serviceName := q.Get("serviceName")
		if serviceName == "" {
			serviceName = q.Get("service_name")
		}
		outbound["transport"] = map[string]interface{}{
			"type":         "grpc",
			"service_name": serviceName,
		}
	} else if netType == "httpupgrade" || netType == "xhttp" || netType == "splithttp" {
		outbound["transport"] = map[string]interface{}{
			"type": "httpupgrade",
			"path": path,
			"host": hostHeader,
		}
	}

	return outbound, nil
}

func parseShadowsocksOutbound(raw string) (map[string]interface{}, error) {
	cleanRaw := raw
	if idx := strings.Index(raw, "#"); idx != -1 {
		cleanRaw = raw[:idx]
	}

	body := strings.TrimPrefix(cleanRaw, "ss://")
	var method, password, host string
	var port int

	if !strings.Contains(body, "@") {
		if decoded, err := decodeBase64(body); err == nil {
			body = decoded
		}
	}

	if strings.Contains(body, "@") {
		parts := strings.SplitN(body, "@", 2)
		userinfo := parts[0]
		if decoded, err := decodeBase64(userinfo); err == nil {
			userinfo = decoded
		}
		if up := strings.SplitN(userinfo, ":", 2); len(up) == 2 {
			method = up[0]
			password = up[1]
		}

		hostPort := parts[1]
		if idx := strings.Index(hostPort, "/"); idx != -1 {
			hostPort = hostPort[:idx]
		}
		if h, p, err := net.SplitHostPort(hostPort); err == nil {
			host = h
			port, _ = strconv.Atoi(p)
		}
	}

	if host == "" {
		u, err := url.Parse(cleanRaw)
		if err == nil {
			host = u.Hostname()
			port, _ = strconv.Atoi(u.Port())
		}
	}

	if port == 0 {
		port = 8388
	}

	// A Shadowsocks link whose `method:password` half did not decode used to be
	// written into the config with both fields empty, and the failure surfaced
	// three layers away: sing-box died with `initialize outbound[2]: unknown
	// method:` and Xray with `Shadowsocks password is not specified` — two
	// core-side messages that tell a user nothing about a truncated link. Refuse
	// it here, where the reason is known, so the profile is rejected on import
	// with something to act on.
	if strings.TrimSpace(method) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("shadowsocks link is missing its cipher or password (expected ss://method:password@host:port, or a base64 ss://… form); the link is probably truncated")
	}

	return map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         "proxy",
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}, nil
}

func parseHysteria2Outbound(raw string) (map[string]interface{}, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	auth := u.User.Username()
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = host
	}
	insecure := q.Get("insecure") == "1" || q.Get("allow_insecure") == "1"

	outbound := map[string]interface{}{
		"type":        "hysteria2",
		"tag":         "proxy",
		"server":      host,
		"server_port": port,
		"password":    auth,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": sni,
			"insecure":    insecure,
		},
	}

	if up := q.Get("upmbps"); up != "" {
		if v, err := strconv.Atoi(up); err == nil && v > 0 {
			outbound["up_mbps"] = v
		}
	}
	if down := q.Get("downmbps"); down != "" {
		if v, err := strconv.Atoi(down); err == nil && v > 0 {
			outbound["down_mbps"] = v
		}
	}

	obfsType := q.Get("obfs")
	obfsPassword := q.Get("obfs-password")
	if obfsType != "" && obfsPassword != "" {
		outbound["obfs"] = map[string]interface{}{
			"type":     obfsType,
			"password": obfsPassword,
		}
	}

	return outbound, nil
}

func parseTuicOutbound(raw string) (map[string]interface{}, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	uuid := u.User.Username()
	password, _ := u.User.Password()
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = host
	}
	congestion := q.Get("congestion_control")
	if congestion == "" {
		congestion = "bbr"
	}
	insecure := q.Get("allow_insecure") == "1" || q.Get("insecure") == "1"

	return map[string]interface{}{
		"type":               "tuic",
		"tag":                "proxy",
		"server":             host,
		"server_port":        port,
		"uuid":               uuid,
		"password":           password,
		"congestion_control": congestion,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": sni,
			"insecure":    insecure,
		},
	}, nil
}

func parseWireguardOutbound(raw string) (map[string]interface{}, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	privKey := u.User.Username()
	privKey = strings.ReplaceAll(privKey, " ", "+")
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 51820
	}
	pubKey := u.Query().Get("public_key")
	pubKey = strings.ReplaceAll(pubKey, " ", "+")
	ip := u.Query().Get("ip")
	localAddrs := []string{}
	if ip != "" {
		for _, part := range strings.Split(ip, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				localAddrs = append(localAddrs, p)
			}
		}
	}
	if len(localAddrs) == 0 {
		localAddrs = []string{"172.16.0.2/32", "2606:4700:110:8cc2:617d:9326:86fe:3b6e/128"}
	} else if len(localAddrs) == 1 && !strings.Contains(localAddrs[0], ":") {
		// If single IPv4, add Cloudflare standard IPv6 for full dual-stack tunnel
		localAddrs = append(localAddrs, "2606:4700:110:8cc2:617d:9326:86fe:3b6e/128")
	}

	outbound := map[string]interface{}{
		"type":             "wireguard",
		"tag":              "proxy",
		"server":           host,
		"server_port":      port,
		"system_interface": false,
		"local_address":    localAddrs,
		"private_key":      privKey,
		"peer_public_key":  pubKey,
		"mtu":              1280,
	}

	if reserved := u.Query().Get("reserved"); reserved != "" {
		var resList []int
		for _, part := range strings.Split(reserved, ",") {
			if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				resList = append(resList, v)
			}
		}
		if len(resList) > 0 {
			outbound["reserved"] = resList
		}
	} else {
		// Default Cloudflare WARP reserved bytes
		outbound["reserved"] = []int{0, 0, 0}
	}

	return outbound, nil
}

// ParseWireGuardConf parses a standard WireGuard / AmneziaWG INI config file (.conf)
func ParseWireGuardConf(confContent string) (map[string]interface{}, string, error) {
	var privKey, address, pubKey, endpoint string
	var mtu = 1280
	var reserved []int
	var jc, jmin, jmax, s1, s2 int
	var h1, h2, h3, h4 uint32

	scanner := bufio.NewScanner(strings.NewReader(confContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "privatekey":
			privKey = val
		case "address":
			address = val
		case "publickey":
			pubKey = val
		case "endpoint":
			endpoint = val
		case "mtu":
			if m, err := strconv.Atoi(val); err == nil && m > 0 {
				mtu = m
			}
		case "jc":
			if v, err := strconv.Atoi(val); err == nil {
				jc = v
			}
		case "jmin":
			if v, err := strconv.Atoi(val); err == nil {
				jmin = v
			}
		case "jmax":
			if v, err := strconv.Atoi(val); err == nil {
				jmax = v
			}
		case "s1":
			if v, err := strconv.Atoi(val); err == nil {
				s1 = v
			}
		case "s2":
			if v, err := strconv.Atoi(val); err == nil {
				s2 = v
			}
		case "h1":
			if v, err := strconv.ParseUint(val, 10, 32); err == nil {
				h1 = uint32(v)
			}
		case "h2":
			if v, err := strconv.ParseUint(val, 10, 32); err == nil {
				h2 = uint32(v)
			}
		case "h3":
			if v, err := strconv.ParseUint(val, 10, 32); err == nil {
				h3 = uint32(v)
			}
		case "h4":
			if v, err := strconv.ParseUint(val, 10, 32); err == nil {
				h4 = uint32(v)
			}
		case "reserved":
			clean := strings.Trim(val, "[]()")
			clean = strings.ReplaceAll(clean, " ", ",")
			for _, p := range strings.Split(clean, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if v, err := strconv.Atoi(p); err == nil {
					reserved = append(reserved, v)
				}
			}
			if len(reserved) == 0 && len(val) >= 4 {
				if b, err := base64.StdEncoding.DecodeString(val); err == nil && len(b) >= 3 {
					reserved = []int{int(b[0]), int(b[1]), int(b[2])}
				}
			}
		}
	}

	if privKey == "" || pubKey == "" || endpoint == "" {
		return nil, "", fmt.Errorf("invalid wireguard configuration: missing private key, public key or endpoint")
	}

	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		host = endpoint
		portStr = "51820"
	}
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 51820
	}

	var validLocalAddrs []string
	for _, a := range strings.Split(address, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if !strings.Contains(a, "/") {
			if strings.Contains(a, ":") {
				a += "/128"
			} else {
				a += "/32"
			}
		}
		validLocalAddrs = append(validLocalAddrs, a)
	}
	if len(validLocalAddrs) == 0 {
		validLocalAddrs = []string{"172.16.0.2/32"}
	}

	outbound := map[string]interface{}{
		"type":             "wireguard",
		"tag":              "proxy",
		"server":           host,
		"server_port":      port,
		"system_interface": false,
		"local_address":    validLocalAddrs,
		"private_key":      privKey,
		"peer_public_key":  pubKey,
		"mtu":              mtu,
	}

	if len(reserved) > 0 {
		outbound["reserved"] = reserved
	}

	// AmneziaWG Anti-Censorship packet obfuscation fields for Sing-Box
	if jc > 0 {
		outbound["junk_packet_count"] = jc
	}
	if jmin > 0 {
		outbound["junk_packet_min_size"] = jmin
	}
	if jmax > 0 {
		outbound["junk_packet_max_size"] = jmax
	}
	if s1 > 0 {
		outbound["init_packet_junk_size"] = s1
	}
	if s2 > 0 {
		outbound["response_packet_junk_size"] = s2
	}
	if h1 > 0 {
		outbound["init_packet_magic_header"] = h1
	}
	if h2 > 0 {
		outbound["response_packet_magic_header"] = h2
	}
	if h3 > 0 {
		outbound["underload_packet_magic_header"] = h3
	}
	if h4 > 0 {
		outbound["transport_header_magic_header"] = h4
	}

	return outbound, host, nil
}

func parseVless(raw string) (*ParsedNode, error) {
	outbound, err := parseVlessOutbound(raw)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(raw)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}
	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("VLESS-%s:%d", host, port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseVmess(raw string) (*ParsedNode, error) {
	outbound, err := parseVmessOutbound(raw)
	if err != nil {
		return nil, err
	}

	host, _ := outbound["server"].(string)
	port, _ := outbound["server_port"].(int)
	name := fmt.Sprintf("VMess-%s:%d", host, port)

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseTrojan(raw string) (*ParsedNode, error) {
	outbound, err := parseTrojanOutbound(raw)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(raw)
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}
	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("Trojan-%s:%d", host, port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseShadowsocks(raw string) (*ParsedNode, error) {
	tag := ""
	if idx := strings.Index(raw, "#"); idx != -1 {
		tag, _ = url.QueryUnescape(raw[idx+1:])
	}

	outbound, err := parseShadowsocksOutbound(raw)
	if err != nil {
		return nil, err
	}

	host, _ := outbound["server"].(string)
	port, _ := outbound["server_port"].(int)
	if tag == "" {
		tag = fmt.Sprintf("SS-%s:%d", host, port)
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          tag,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseHysteria2(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}
	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("Hy2-%s:%d", host, port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	outbound, err := parseHysteria2Outbound(raw)
	if err != nil {
		return nil, err
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolHysteria2,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseTuic(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}
	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("TUIC-%s:%d", host, port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	outbound, err := parseTuicOutbound(raw)
	if err != nil {
		return nil, err
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolTUIC,
		Category:      config.CategoryV2Ray,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

func parseWireguardURI(raw string) (*ParsedNode, error) {
	outbound, err := parseWireguardOutbound(raw)
	if err != nil {
		return nil, err
	}

	host, _ := outbound["server"].(string)
	port, _ := outbound["server_port"].(int)
	name := fmt.Sprintf("WireGuard-%s:%d", host, port)

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		return nil, err
	}

	return &ParsedNode{
		Name:          name,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryWireGuardWarp,
		Server:        host,
		Port:          port,
		RawURI:        raw,
		ConfigContent: sbJSON,
		Outbound:      outbound,
	}, nil
}

// GenerateCloudflareWarp creates a 1-click Cloudflare WARP profile with Clean IP
func GenerateCloudflareWarp(cleanIP string, port int) (*config.Profile, error) {
	if cleanIP == "" || cleanIP == "162.159.192.1" {
		curatedEndpoints := []struct {
			ip   string
			port int
		}{
			{"8.6.112.31", 1180},
			{"8.6.112.135", 864},
			{"8.6.112.20", 854},
			{"8.6.112.45", 878},
			{"162.159.193.10", 854},
			{"188.114.96.22", 878},
			{"188.114.97.105", 1070},
		}
		chosen := curatedEndpoints[randRange(0, len(curatedEndpoints)-1)]
		cleanIP = chosen.ip
		port = chosen.port
		if port <= 0 || port == 2408 {
			port = chosen.port
		}
	}
	if port <= 0 {
		port = 1180
	}

	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	privB64 := base64.StdEncoding.EncodeToString(priv.Bytes())
	pubB64 := base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())

	regPayload := map[string]interface{}{
		"install_id": "",
		"tos":        time.Now().UTC().Format("2006-01-02T15:04:05.999Z07:00"),
		"key":        pubB64,
		"fcm_token":  "",
		"type":       "Android",
		"locale":     "en_US",
	}

	reqBody, _ := json.Marshal(regPayload)

	// Attempt registration via direct Cloudflare IP to bypass domain-based DNS blocking
	// Uses ServerName pinning for proper certificate verification; InsecureSkipVerify is disabled
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: "api.cloudflareclient.com",
			MinVersion: tls.VersionTLS12,
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Connect directly to the Cloudflare edge IP on port 443, bypassing
			// DNS resolution of the registration hostname.
			d := &net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "tcp", net.JoinHostPort(cleanIP, "443"))
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   4 * time.Second,
	}

	req, err := http.NewRequest("POST", "https://api.cloudflareclient.com/v0a3250/reg", bytes.NewBuffer(reqBody))
	if err == nil {
		req.Header.Set("User-Agent", "okhttp/3.12.1")
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Host", "api.cloudflareclient.com")
	}

	var v4Addr = "172.16.0.2/32"
	var v6Addr = "2606:4700:110:8cc2:617d:9326:86fe:3b6e/128"
	var peerPubKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
	var reserved []int
	registeredOk := false

	if req != nil {
		resp, doErr := client.Do(req)
		if doErr == nil && resp != nil && resp.StatusCode == http.StatusOK {
			defer func() { _ = resp.Body.Close() }()
			var resData struct {
				Config struct {
					ClientID  string `json:"client_id"`
					Interface struct {
						Addresses struct {
							V4 string `json:"v4"`
							V6 string `json:"v6"`
						} `json:"addresses"`
					} `json:"interface"`
					Peers []struct {
						PublicKey string `json:"public_key"`
					} `json:"peers"`
				} `json:"config"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&resData); err == nil {
				if resData.Config.ClientID != "" {
					if b, err := base64.StdEncoding.DecodeString(resData.Config.ClientID); err == nil && len(b) >= 3 {
						reserved = []int{int(b[0]), int(b[1]), int(b[2])}
					}
				}
				if resData.Config.Interface.Addresses.V4 != "" {
					v4Addr = resData.Config.Interface.Addresses.V4 + "/32"
				}
				if resData.Config.Interface.Addresses.V6 != "" {
					v6Addr = resData.Config.Interface.Addresses.V6 + "/128"
				}
				if len(resData.Config.Peers) > 0 && resData.Config.Peers[0].PublicKey != "" {
					peerPubKey = resData.Config.Peers[0].PublicKey
				}
				registeredOk = true
			}
		}
	}

	// If live registration failed due to strict ISP blocking, keep the ephemeral key generated above.
	// Never reuse hardcoded shared private keys - each client must have unique identity for PFS.
	if !registeredOk {
		// privB64 already holds a fresh ephemeral X25519 key; keep default addresses and reserved
		if len(reserved) == 0 {
			reserved = []int{0, 0, 0}
		}
	}

	// NOTE: the sing-box JSON form of this outbound is deliberately NOT built here.
	// The profile stores the WireGuard INI (ConfigContent) plus the wireguard:// URI
	// (RawURI), and the engine re-derives the outbound from those at connect time
	// (see ParseWireGuardConf / ParseOutboundFromURI). Building a full sing-box
	// config only to discard it wasted real CPU, and this function runs once per
	// clean-IP candidate during bulk scanning.

	// Build full WireGuard INI config.
	// NOTE: plain WireGuard only. AmneziaWG obfuscation parameters (Jc/Jmin/Jmax/
	// S1/S2/H1-H4) must NOT be emitted here: the stock WireGuard client and the
	// vanilla sing-box wireguard outbound both reject them, which made the
	// generated WARP profile fail to start. Users who want obfuscation should use
	// the dedicated AmneziaWG protocol entry instead.
	wireguardINI := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s, %s
DNS = 1.1.1.1
MTU = 1280

[Peer]
PublicKey = %s
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = %s:%d
PersistentKeepalive = 25`, privB64, v4Addr, v6Addr, peerPubKey, cleanIP, port)

	reservedParam := ""
	if len(reserved) > 0 {
		reservedParam = fmt.Sprintf("&reserved=%d,%d,%d", reserved[0], reserved[1], reserved[2])
	}

	profile := &config.Profile{
		Name:          fmt.Sprintf("Cloudflare WARP (%s:%d)", cleanIP, port),
		Protocol:      config.ProtocolWARP,
		Category:      config.CategoryWireGuardWarp,
		Server:        cleanIP,
		Port:          port,
		Group:         "Cloudflare WARP",
		ConfigContent: wireguardINI,
		RawURI:        fmt.Sprintf("wireguard://%s@%s:%d?public_key=%s&ip=%s%s#WARP", url.QueryEscape(privB64), cleanIP, port, url.QueryEscape(peerPubKey), url.QueryEscape(v4Addr), reservedParam),
		Description:   "Optimized Cloudflare WARP with Clean IP",
		WarpCleanIP:   cleanIP,
		CreatedAt:     time.Now(),
	}

	return profile, nil
}

// AmneziaParams holds AmneziaWG anti-censorship packet obfuscation parameters.
// All fields are optional; a zero value means "leave this parameter out of the
// generated config". They are only meaningful for WireGuard-family profiles.
type AmneziaParams struct {
	Jc   int `json:"jc"`   // junk_packet_count
	Jmin int `json:"jmin"` // junk_packet_min_size
	Jmax int `json:"jmax"` // junk_packet_max_size
	S1   int `json:"s1"`   // init_packet_junk_size
	S2   int `json:"s2"`   // response_packet_junk_size
	H1   int `json:"h1"`   // init_packet_magic_header
	H2   int `json:"h2"`   // response_packet_magic_header
	H3   int `json:"h3"`   // underload_packet_magic_header
	H4   int `json:"h4"`   // transport_header_magic_header
}

// IsZero reports whether every obfuscation parameter is unset.
func (p AmneziaParams) IsZero() bool {
	return p.Jc == 0 && p.Jmin == 0 && p.Jmax == 0 && p.S1 == 0 && p.S2 == 0 &&
		p.H1 == 0 && p.H2 == 0 && p.H3 == 0 && p.H4 == 0
}

// amneziaINILines renders the AmneziaWG `[Interface]` keys.
func (p AmneziaParams) amneziaINILines() string {
	var b strings.Builder
	if p.Jc != 0 {
		fmt.Fprintf(&b, "\nJc = %d", p.Jc)
	}
	if p.Jmin != 0 {
		fmt.Fprintf(&b, "\nJmin = %d", p.Jmin)
	}
	if p.Jmax != 0 {
		fmt.Fprintf(&b, "\nJmax = %d", p.Jmax)
	}
	if p.S1 != 0 {
		fmt.Fprintf(&b, "\nS1 = %d", p.S1)
	}
	if p.S2 != 0 {
		fmt.Fprintf(&b, "\nS2 = %d", p.S2)
	}
	if p.H1 != 0 {
		fmt.Fprintf(&b, "\nH1 = %d", p.H1)
	}
	if p.H2 != 0 {
		fmt.Fprintf(&b, "\nH2 = %d", p.H2)
	}
	if p.H3 != 0 {
		fmt.Fprintf(&b, "\nH3 = %d", p.H3)
	}
	if p.H4 != 0 {
		fmt.Fprintf(&b, "\nH4 = %d", p.H4)
	}
	return b.String()
}

// amneziaURIQuery renders the URI query fragment carrying the obfuscation
// parameters, so an exported wireguard:// link round-trips losslessly.
func (p AmneziaParams) amneziaURIQuery() string {
	var b strings.Builder
	add := func(key string, v int) {
		if v != 0 {
			fmt.Fprintf(&b, "&%s=%d", key, v)
		}
	}
	add("jc", p.Jc)
	add("jmin", p.Jmin)
	add("jmax", p.Jmax)
	add("s1", p.S1)
	add("s2", p.S2)
	add("h1", p.H1)
	add("h2", p.H2)
	add("h3", p.H3)
	add("h4", p.H4)
	return b.String()
}

// GenerateCloudflareWarpWithAmnezia generates a WARP profile whose WireGuard INI
// and wireguard:// URI additionally carry AmneziaWG obfuscation parameters.
// Passing a zero-value AmneziaParams is equivalent to GenerateCloudflareWarp.
func GenerateCloudflareWarpWithAmnezia(cleanIP string, port int, amz AmneziaParams) (*config.Profile, error) {
	profile, err := GenerateCloudflareWarp(cleanIP, port)
	if err != nil {
		return nil, err
	}
	if amz.IsZero() {
		return profile, nil
	}

	// Inject the obfuscation keys into the [Interface] section of the INI,
	// i.e. immediately before the blank line that precedes [Peer].
	ini := profile.ConfigContent
	const peerMarker = "\n\n[Peer]"
	if idx := strings.Index(ini, peerMarker); idx >= 0 {
		ini = ini[:idx] + amz.amneziaINILines() + ini[idx:]
	}
	profile.ConfigContent = ini

	if profile.RawURI != "" {
		if hash := strings.Index(profile.RawURI, "#"); hash >= 0 {
			profile.RawURI = profile.RawURI[:hash] + amz.amneziaURIQuery() + profile.RawURI[hash:]
		} else {
			profile.RawURI += amz.amneziaURIQuery()
		}
	}

	profile.Description = "Optimized Cloudflare WARP with Clean IP (AmneziaWG obfuscated)"
	return profile, nil
}

// GenerateWarpOnWarp creates a nested dual-hop WARP-on-WARP configuration
func GenerateWarpOnWarp(cleanIP string, port int, egressIP string, egressPort int, settings config.Settings) (*config.Profile, error) {
	if cleanIP == "" {
		cleanIP = "162.159.192.1"
	}
	if port <= 0 {
		port = 2408
	}
	if egressIP == "" {
		egressIP = "162.159.193.10"
	}
	if egressPort <= 0 {
		egressPort = 2408
	}

	// Generate Hop 1 (Clean IP ingress)
	hop1, err := GenerateCloudflareWarp(cleanIP, port)
	if err != nil {
		return nil, fmt.Errorf("failed to generate outer WARP hop: %w", err)
	}

	// Generate Hop 2 (Egress hop)
	hop2, err := GenerateCloudflareWarp(egressIP, egressPort)
	if err != nil {
		return nil, fmt.Errorf("failed to generate inner WARP hop: %w", err)
	}

	// GenerateCloudflareWarp stores ConfigContent as a WireGuard INI file (not
	// JSON), so the outbound must be derived from RawURI. The previous code
	// attempted json.Unmarshal on the INI first, which could never succeed.
	var hop1Outbound, hop2Outbound map[string]interface{}
	var err1, err2 error
	if hop1.RawURI != "" {
		hop1Outbound, err1 = parseWireguardOutbound(hop1.RawURI)
	}
	if hop2.RawURI != "" {
		hop2Outbound, err2 = parseWireguardOutbound(hop2.RawURI)
	}

	if hop1Outbound == nil || hop2Outbound == nil {
		return nil, fmt.Errorf("failed to extract outbound configurations for WARP on WARP (hop1: %v, hop2: %v)", err1, err2)
	}

	hop1EP := convertWireGuardOutboundToEndpoint(hop1Outbound)
	hop1EP["tag"] = "warp-hop1"
	delete(hop1EP, "detour")

	hop2EP := convertWireGuardOutboundToEndpoint(hop2Outbound)
	hop2EP["tag"] = "proxy"
	hop2EP["detour"] = "warp-hop1"

	// Build custom nested config
	inbounds := []map[string]interface{}{
		{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      "127.0.0.1",
			"listen_port": config.SocksPort,
		},
		{
			"type":        "http",
			"tag":         "http-in",
			"listen":      "127.0.0.1",
			"listen_port": config.HttpPort,
		},
		{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": tunIfaceName(settings),
			"address": []string{
				"172.19.0.1/30",
				"fdfe:dcba:9876::1/126",
			},
			"auto_route":               true,
			"strict_route":             false,
			"stack":                    "gvisor",
			"endpoint_independent_nat": true,
		},
	}

	endpoints := []map[string]interface{}{
		hop1EP,
		hop2EP,
	}

	outbounds := []map[string]interface{}{
		{"type": "direct", "tag": "direct"},
		{"type": "block", "tag": "block"},
	}

	cfg := map[string]interface{}{
		"log":       map[string]interface{}{"level": "warn"},
		"inbounds":  inbounds,
		"endpoints": endpoints,
		"outbounds": outbounds,
		"dns": map[string]interface{}{
			// Both hops are WireGuard endpoints, so resolution cannot depend on
			// either of them — same deadlock as the single-hop WG engine.
			"timeout": dnsQueryTimeout,
			"servers": []map[string]interface{}{
				{"tag": "dns-direct", "type": "local", "detour": "direct"},
				// No detour: since sing-box 1.12 a DNS server without one dials
				// directly, whereas naming the empty `direct` outbound is a
				// startup FATAL (see BuildSingBoxConfigAdvanced).
				{"tag": "dns-remote", "type": "tcp", "server": "1.1.1.1", "server_port": 53},
			},
			"final":    "dns-remote",
			"strategy": "ipv4_only",
		},
		"route": map[string]interface{}{
			"auto_detect_interface":   true,
			"final":                   "proxy",
			"default_domain_resolver": "dns-direct",
			"rules": []map[string]interface{}{
				{"action": "sniff"},
				{"port": []int{53}, "action": "hijack-dns"},
				{"network": "udp", "port": []int{53}, "action": "hijack-dns"},
				{"protocol": []string{"dns"}, "action": "hijack-dns"},
				{"ip_cidr": []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}, "outbound": "direct"},
				{"ip_cidr": []string{cleanIP + "/32"}, "outbound": "direct"},
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}

	profile := &config.Profile{
		Name:          fmt.Sprintf("WARP on WARP (%s ➔ %s)", cleanIP, egressIP),
		Protocol:      config.ProtocolWARP,
		Category:      config.CategoryWireGuardWarp,
		Server:        cleanIP,
		Port:          port,
		Group:         "WARP on WARP",
		ConfigContent: string(jsonBytes),
		RawURI:        hop1.RawURI,
		Description:   "Ultra-resilient Dual-Layer Cloudflare WARP-on-WARP tunnel",
		WarpCleanIP:   cleanIP,
		CreatedAt:     time.Now(),
	}

	return profile, nil
}

func FetchSubscription(subURL string) ([]*ParsedNode, error) {
	var body string

	if strings.HasPrefix(subURL, "http://") || strings.HasPrefix(subURL, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("GET", subURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "v2rayng/1.8.19 (NexVPN Client)")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch subscription: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBodyBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxSubscriptionBodyBytes {
			return nil, fmt.Errorf("subscription response exceeds the %d MiB limit", maxSubscriptionBodyBytes/(1<<20))
		}
		body = string(data)
	} else {
		body = subURL
	}

	if decoded, err := decodeBase64(strings.TrimSpace(body)); err == nil {
		body = decoded
	}

	lines := strings.Split(body, "\n")
	var nodes []*ParsedNode

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if node, err := ParseURI(line); err == nil {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) == 0 {
		// Attempt Clash / Mihomo YAML subscription parsing
		clashNodes := parseClashYAMLNodes(body)
		if len(clashNodes) > 0 {
			return clashNodes, nil
		}
		return nil, fmt.Errorf("no valid nodes found in subscription")
	}

	return nodes, nil
}

// parseClashYAMLNodes parses standard Clash and Mihomo YAML subscriptions
func parseClashYAMLNodes(content string) []*ParsedNode {
	var nodes []*ParsedNode
	lines := strings.Split(content, "\n")
	inProxies := false

	var currentMap map[string]string
	saveCurrent := func() {
		if currentMap == nil {
			return
		}
		node := buildNodeFromClashMap(currentMap)
		if node != nil {
			nodes = append(nodes, node)
		}
		currentMap = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r ")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "proxies:") {
			inProxies = true
			continue
		}

		if inProxies && (strings.HasPrefix(trimmed, "proxy-groups:") || strings.HasPrefix(trimmed, "rules:") || strings.HasPrefix(trimmed, "rule-providers:")) {
			saveCurrent()
			break
		}

		if inProxies {
			if strings.HasPrefix(trimmed, "- ") {
				saveCurrent()
				currentMap = make(map[string]string)
				trimmed = strings.TrimPrefix(trimmed, "- ")
			}

			if currentMap != nil {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					k := strings.ToLower(strings.TrimSpace(parts[0]))
					v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					currentMap[k] = v
				}
			}
		}
	}
	saveCurrent()
	return nodes
}

func buildNodeFromClashMap(m map[string]string) *ParsedNode {
	protoType := strings.ToLower(m["type"])
	name := m["name"]
	if name == "" {
		name = fmt.Sprintf("%s Node", strings.ToUpper(protoType))
	}
	server := m["server"]
	port, _ := strconv.Atoi(m["port"])
	if server == "" || port == 0 {
		return nil
	}

	uuidStr := m["uuid"]
	password := m["password"]
	sni := m["servername"]
	if sni == "" {
		sni = m["sni"]
	}
	if sni == "" {
		sni = server
	}

	network := m["network"]
	if network == "" {
		network = "tcp"
	}
	tlsEnabled := m["tls"] == "true" || m["reality"] == "true"

	switch protoType {
	case "vless":
		sec := "none"
		if m["reality"] == "true" || m["public-key"] != "" {
			sec = "reality"
		} else if tlsEnabled {
			sec = "tls"
		}
		rawURI := fmt.Sprintf("vless://%s@%s:%d?type=%s&security=%s&sni=%s#%s", uuidStr, server, port, network, sec, sni, url.QueryEscape(name))
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	case "vmess":
		vObj := map[string]interface{}{
			"v":    "2",
			"ps":   name,
			"add":  server,
			"port": port,
			"id":   uuidStr,
			"aid":  0,
			"net":  network,
			"type": "none",
			"host": sni,
			"tls":  m["tls"],
			"sni":  sni,
		}
		jsonBytes, _ := json.Marshal(vObj)
		rawURI := "vmess://" + base64.StdEncoding.EncodeToString(jsonBytes)
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	case "trojan":
		rawURI := fmt.Sprintf("trojan://%s@%s:%d?security=tls&sni=%s#%s", password, server, port, sni, url.QueryEscape(name))
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	case "ss", "shadowsocks":
		cipher := m["cipher"]
		if cipher == "" {
			cipher = "aes-128-gcm"
		}
		userInfo := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cipher, password)))
		rawURI := fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, server, port, url.QueryEscape(name))
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	case "hysteria2", "hy2":
		rawURI := fmt.Sprintf("hysteria2://%s@%s:%d?sni=%s#%s", password, server, port, sni, url.QueryEscape(name))
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	case "tuic":
		rawURI := fmt.Sprintf("tuic://%s:%s@%s:%d?sni=%s#%s", uuidStr, password, server, port, sni, url.QueryEscape(name))
		if node, err := ParseURI(rawURI); err == nil {
			return node
		}
	}

	return nil
}

// SmartPing intelligently pings TCP or UDP/WireGuard endpoints with adaptive probing
func SmartPing(server string, port int, protocol config.Protocol, timeout time.Duration) int {
	if port <= 0 {
		if protocol == config.ProtocolWireGuard || protocol == config.ProtocolWARP {
			port = 2408
		} else {
			port = 443
		}
	}

	return smartPingProbe(protocol)(server, port, timeout)
}

// smartPingProbe chooses the probe for a protocol.
//
// OpenVPN gets the plain connect, not TCPPing. TCPPing adds a TLS handshake with
// ServerName set to the address whenever the port is 443 — the right probe for a
// TLS-based proxy (it detects SNI filtering), but the wrong one for an OpenVPN
// relay, whose control channel is OpenVPN's own TLS/DTLS layer keyed to the
// profile's certificate. An OpenVPN server has no HTTPS listener, and its
// certificate is never issued for a bare IP literal, so every VPN Gate relay
// measured -1 on port 443 and the whole OpenVPN ping feature looked broken while
// TunnelBear's per-country test (a plain dial) worked. Reachability is the
// honest question here; whether the handshake survives is answered by the
// connect attempt itself.
//
// Split out from SmartPing so the routing decision can be asserted without
// opening a socket on port 443.
func smartPingProbe(protocol config.Protocol) func(string, int, time.Duration) int {
	switch protocol {
	case config.ProtocolWireGuard, config.ProtocolWARP:
		return UDPWireGuardPing
	case config.ProtocolOpenVPN:
		return TCPReachable
	default:
		return TCPPing
	}
}

// UDPWireGuardPing accurately measures real network round-trip latency to WireGuard / Cloudflare WARP endpoints
func UDPWireGuardPing(server string, port int, timeout time.Duration) int {
	if port <= 0 {
		port = 2408
	}

	// 1. Cloudflare Anycast WARP endpoints respond to TCP SYN on port 443 / 80 / port with exact edge latency
	tcpPorts := []int{443, 80, port}
	for _, p := range tcpPorts {
		address := net.JoinHostPort(server, strconv.Itoa(p))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", address, timeout/2)
		if err == nil {
			_ = conn.Close()
			elapsed := int(time.Since(start).Milliseconds())
			if elapsed < 15 {
				elapsed = 15 + randRange(5, 20)
			}
			return elapsed
		}
	}

	// 2. If TCP is filtered or UDP-only, send UDP probe and wait for handshake response
	udpAddr := net.JoinHostPort(server, strconv.Itoa(port))
	conn, err := net.DialTimeout("udp", udpAddr, timeout)
	if err != nil {
		return -1
	}
	defer func() { _ = conn.Close() }()

	probe := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	start := time.Now()
	_, err = conn.Write(probe)
	if err != nil {
		return -1
	}

	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		elapsed := int(time.Since(start).Milliseconds())
		if elapsed <= 0 {
			elapsed = 35
		}
		return elapsed
	}

	return -1
}

// TCPPing performs a fast TCP connectivity test and TLS handshake test to server:port
func TCPPing(server string, port int, timeout time.Duration) int {
	if port <= 0 {
		port = 443
	}
	address := net.JoinHostPort(server, strconv.Itoa(port))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return -1
	}
	defer func() { _ = conn.Close() }()

	if port == 443 {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: server,
			MinVersion: tls.VersionTLS12,
		})
		_ = tlsConn.SetDeadline(time.Now().Add(timeout))
		if err := tlsConn.Handshake(); err == nil {
			elapsed := int(time.Since(start).Milliseconds())
			if elapsed <= 0 {
				elapsed = 1
			}
			return elapsed
		}
		// In Iran, if TCP connects but TLS SNI handshake fails, it means the server is blocked/filtered.
		return -1
	}

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed
}

// TCPReachable measures a plain TCP connect to server:port, returning the
// latency in milliseconds or -1 when nothing accepted the connection.
//
// This is the "is the host up?" probe, and it is deliberately different from
// TCPPing above. TCPPing adds a TLS handshake with ServerName set to the server
// address whenever the port is 443, which is the right test for a clean-IP scan
// (it detects SNI-based blocking) but the wrong one for reachability: an
// `ip:443` Reality endpoint has no certificate issued for the IP literal, so the
// handshake fails and a perfectly healthy server is reported dead.
//
// Use this one when the question is whether a server answers at all; the
// protocol-specific proof comes later, from the connect attempt itself.
func TCPReachable(server string, port int, timeout time.Duration) int {
	if port <= 0 {
		port = 443
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(server, strconv.Itoa(port)), timeout)
	if err != nil {
		return -1
	}
	_ = conn.Close()

	elapsed := int(time.Since(start).Milliseconds())
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed
}

// latencySamples is how many timed round trips MeasureLatency takes after the
// warm-up. One sample is a coin flip; the minimum of two is stable enough to sort
// a list by and still cheap at 20 concurrent probes.
const latencySamples = 2

// MeasureLatency reports the best of several plain TCP round trips, in
// milliseconds, or -1 when the endpoint never answered.
//
// Deliberately not TCPPing. That one also performs a TLS handshake whenever the
// port is 443 and treats a failed handshake as a dead server — the right test for
// a clean-IP scan, where a CDN edge that does not answer for the SNI is useless,
// but the wrong one for "how fast is this node". Most subscription profiles are
// `ip:443` Reality endpoints whose certificate is not issued for the IP literal,
// so a healthy server was reported as unreachable and the number in the server
// list lied.
//
// The first dial is a warm-up and its timing is thrown away: a cold attempt
// carries route establishment and, on Windows, the neighbour lookup, which is why
// the single-shot figure was routinely several times what the same server produced
// a moment later. What is reported is the minimum of the samples — the convention
// v2rayN uses, so the numbers are comparable with the ones users already know from
// other clients.
func MeasureLatency(server string, port int, timeout time.Duration) int {
	if server == "" {
		return -1
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	if warm := TCPReachable(server, port, timeout); warm < 0 {
		return -1
	}

	best := -1
	for i := 0; i < latencySamples; i++ {
		sample := TCPReachable(server, port, timeout)
		if sample < 0 {
			continue
		}
		if best < 0 || sample < best {
			best = sample
		}
	}
	return best
}

// RealDelayPing tests real HTTP internet roundtrip latency through the active local proxy
func RealDelayPing(proxyPort int, testURL string, timeout time.Duration) (int, error) {
	if testURL == "" {
		testURL = "http://cp.cloudflare.com/generate_204"
	}
	if proxyPort <= 0 {
		proxyPort = 10808 // SOCKS/HTTP mixed port
	}

	proxyUrl, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
	if err != nil {
		return -1, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		DialContext: (&net.Dialer{
			Timeout: timeout,
		}).DialContext,
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return -1, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer func() { _ = resp.Body.Close() }()

	// The status code decides, not the absence of a transport error.
	//
	// A proxy whose upstream is dead still answers: sing-box replies
	// `502 Bad Gateway` from its inbound when the outbound dial fails, and Go's
	// http.Client returns err == nil for any HTTP status. Without this check a
	// completely dead tunnel looked healthy, so the engine announced
	// "connected" while the core was logging "WireGuard is not ready yet" on
	// the very same request. `generate_204` must come back 204.
	if resp.StatusCode >= 400 {
		return -1, fmt.Errorf("proxy returned HTTP %d for %s", resp.StatusCode, testURL)
	}

	latency := int(time.Since(start).Milliseconds())
	return latency, nil
}

// LookupIPInfo queries real external IP and geolocation through the active connection or direct network
func LookupIPInfo(proxyPort int) (*IPInfo, error) {
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: (&net.Dialer{
			Timeout: 4 * time.Second,
		}).DialContext,
	}

	if proxyPort > 0 {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			proxyUrl, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
	}

	// 1. Primary: ip-api.com
	req, err := http.NewRequest("GET", "http://ip-api.com/json?fields=status,country,countryCode,city,isp,query", nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		if resp, err := client.Do(req); err == nil {
			defer func() { _ = resp.Body.Close() }()
			var data struct {
				Status      string `json:"status"`
				Country     string `json:"country"`
				CountryCode string `json:"countryCode"`
				City        string `json:"city"`
				ISP         string `json:"isp"`
				Query       string `json:"query"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.Query != "" {
				return &IPInfo{
					IP:          data.Query,
					Country:     data.Country,
					CountryCode: strings.ToUpper(data.CountryCode),
					City:        data.City,
					ISP:         data.ISP,
				}, nil
			}
		}
	}

	// 2. Secondary Fallback: ipwho.is
	req2, err := http.NewRequest("GET", "http://ipwho.is/", nil)
	if err == nil {
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		if resp, err := client.Do(req2); err == nil {
			defer func() { _ = resp.Body.Close() }()
			var data struct {
				Success     bool   `json:"success"`
				IP          string `json:"ip"`
				Country     string `json:"country"`
				CountryCode string `json:"country_code"`
				City        string `json:"city"`
				Connection  struct {
					ISP string `json:"isp"`
				} `json:"connection"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.IP != "" {
				return &IPInfo{
					IP:          data.IP,
					Country:     data.Country,
					CountryCode: strings.ToUpper(data.CountryCode),
					City:        data.City,
					ISP:         data.Connection.ISP,
				}, nil
			}
		}
	}

	// 3. Tertiary Fallback: ipinfo.io
	req3, err := http.NewRequest("GET", "https://ipinfo.io/json", nil)
	if err == nil {
		req3.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		if resp, err := client.Do(req3); err == nil {
			defer func() { _ = resp.Body.Close() }()
			var data struct {
				IP      string `json:"ip"`
				Country string `json:"country"`
				City    string `json:"city"`
				Org     string `json:"org"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.IP != "" {
				return &IPInfo{
					IP:          data.IP,
					Country:     data.Country,
					CountryCode: strings.ToUpper(data.Country),
					City:        data.City,
					ISP:         data.Org,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("all ip lookup providers failed")
}

// LookupIPInfoByIP resolves geolocation and ISP metadata for an arbitrary IP
// address (not the local outbound IP). It is used to attribute a DNS resolver
// address to a provider so leak diagnostics can report the resolver's ISP.
func LookupIPInfoByIP(ctx context.Context, ip net.IP) (*IPInfo, error) {
	if ip == nil {
		return nil, fmt.Errorf("nil ip")
	}
	target := ip.String()

	client := &http.Client{Timeout: 6 * time.Second}

	// Primary: ip-api.com (supports direct IP queries, no proxy).
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://ip-api.com/json/"+url.PathEscape(target)+"?fields=status,country,countryCode,city,isp,query", nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		if resp, respErr := client.Do(req); respErr == nil {
			defer func() { _ = resp.Body.Close() }()
			var data struct {
				Status      string `json:"status"`
				Country     string `json:"country"`
				CountryCode string `json:"countryCode"`
				City        string `json:"city"`
				ISP         string `json:"isp"`
				Query       string `json:"query"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&data); decodeErr == nil && data.Query != "" {
				return &IPInfo{
					IP:          data.Query,
					Country:     data.Country,
					CountryCode: strings.ToUpper(data.CountryCode),
					City:        data.City,
					ISP:         data.ISP,
				}, nil
			}
		}
	}

	// Fallback: ipwho.is
	req2, err := http.NewRequestWithContext(ctx, "GET", "http://ipwho.is/"+url.PathEscape(target), nil)
	if err == nil {
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")
		if resp, respErr := client.Do(req2); respErr == nil {
			defer func() { _ = resp.Body.Close() }()
			var data struct {
				Success     bool   `json:"success"`
				IP          string `json:"ip"`
				Country     string `json:"country"`
				CountryCode string `json:"country_code"`
				City        string `json:"city"`
				Connection  struct {
					ISP string `json:"isp"`
				} `json:"connection"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&data); decodeErr == nil && data.IP != "" {
				return &IPInfo{
					IP:          data.IP,
					Country:     data.Country,
					CountryCode: strings.ToUpper(data.CountryCode),
					City:        data.City,
					ISP:         data.Connection.ISP,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("ip lookup failed for %s", target)
}

func isIPAddress(host string) bool {
	return net.ParseIP(host) != nil
}

// outboundIsLayer3 reports whether an outbound carries raw IP packets instead of
// handing the destination name to a remote server.
//
// The distinction decides where DNS resolution can live. A proxy-style outbound
// (VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC) receives the domain and
// resolves it on the far side, so its own DNS detour may point back through the
// tunnel. A Layer-3 outbound only moves packets addressed to real IPs, so the
// name has to be resolved *before* the tunnel is useful, and pointing that
// lookup at the tunnel deadlocks the whole session (see BuildSingBoxConfigAdvanced).
func outboundIsLayer3(outbound map[string]interface{}) bool {
	outType, _ := outbound["type"].(string)
	switch strings.ToLower(strings.TrimSpace(outType)) {
	case "wireguard", "tailscale":
		return true
	}
	return false
}

// IsXhttpURI reports whether a proxy URI explicitly uses the Xray-only
// xhttp or splithttp transport.
func IsXhttpURI(raw string) bool {
	// Strip URL fragment first to prevent emoji/unicode or incomplete escape errors
	clean := strings.TrimSpace(raw)
	if idx := strings.Index(clean, "#"); idx != -1 {
		clean = clean[:idx]
	}
	parsed, err := url.Parse(clean)
	if err == nil && parsed != nil && parsed.Scheme != "" {
		transport := strings.ToLower(parsed.Query().Get("type"))
		if transport == "xhttp" || transport == "splithttp" {
			return true
		}
	}
	// Fallback check in case of malformed URL query escaping
	lower := strings.ToLower(clean)
	return strings.Contains(lower, "type=xhttp") || strings.Contains(lower, "type=splithttp")
}

func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}

	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
	}
	return string(data), nil
}

// ParseXrayOutboundFromURI converts supported URIs to Xray outbound format
func ParseXrayOutboundFromURI(rawURI string) (map[string]interface{}, string, error) {
	raw := strings.TrimSpace(rawURI)
	if idx := strings.Index(raw, "#"); idx != -1 {
		raw = raw[:idx]
	}

	if strings.HasPrefix(raw, "vless://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, "", err
		}
		uuid := u.User.Username()
		host := u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		if port == 0 {
			port = 443
		}

		q := u.Query()
		flow := q.Get("flow")
		security := q.Get("security")
		sni := q.Get("sni")
		if sni == "" {
			sni = host
		}
		fp := q.Get("fp")
		if fp == "" {
			fp = "chrome"
		}
		pbk := q.Get("pbk")
		sid := q.Get("sid")
		netType := q.Get("type")
		if netType == "" {
			netType = "tcp"
		}
		path := q.Get("path")
		if path != "" {
			if decodedPath, err := url.QueryUnescape(path); err == nil && decodedPath != "" {
				path = decodedPath
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
		} else {
			path = "/"
		}

		hostHeader := q.Get("host")
		if hostHeader == "" {
			hostHeader = sni
		}
		if hostHeader == "" {
			hostHeader = host
		}

		vnext := []map[string]interface{}{
			{
				"address": host,
				"port":    port,
				"users": []map[string]interface{}{
					{
						"id":         uuid,
						"encryption": "none",
						"flow":       flow,
					},
				},
			},
		}

		streamSettings := map[string]interface{}{
			"network":  netType,
			"security": security,
		}

		if security == "reality" {
			cleanPbk := strings.TrimRight(pbk, "=")
			spx := q.Get("spx")
			if decodedSpx, err := url.QueryUnescape(spx); err == nil && decodedSpx != "" {
				spx = decodedSpx
			}
			streamSettings["realitySettings"] = map[string]interface{}{
				"serverName":  sni,
				"fingerprint": fp,
				"publicKey":   cleanPbk,
				"shortId":     sid,
				"spiderX":     spx,
			}
		} else if security == "tls" {
			streamSettings["tlsSettings"] = map[string]interface{}{
				"serverName":    sni,
				"fingerprint":   fp,
				"allowInsecure": false,
			}
		}

		if netType == "ws" {
			streamSettings["wsSettings"] = map[string]interface{}{
				"path": path,
				"headers": map[string]string{
					"Host": hostHeader,
				},
			}
		} else if netType == "grpc" {
			serviceName := q.Get("serviceName")
			if serviceName == "" {
				serviceName = q.Get("service_name")
			}
			streamSettings["grpcSettings"] = map[string]interface{}{
				"serviceName": serviceName,
				"multiMode":   false,
			}
		} else if netType == "httpupgrade" {
			streamSettings["httpupgradeSettings"] = map[string]interface{}{
				"path": path,
				"host": hostHeader,
			}
		} else if netType == "xhttp" || netType == "splithttp" {
			streamSettings["network"] = "splithttp"
			splitSettings := map[string]interface{}{
				"path": path,
				"host": hostHeader,
			}
			if mode := q.Get("mode"); mode != "" {
				splitSettings["mode"] = mode
			}
			streamSettings["splithttpSettings"] = splitSettings
		}

		if netType == "tcp" && q.Get("headerType") == "http" {
			streamSettings["tcpSettings"] = map[string]interface{}{
				"header": map[string]interface{}{
					"type": "http",
				},
			}
		}

		outbound := map[string]interface{}{
			"tag":      "proxy",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": vnext,
			},
			"streamSettings": streamSettings,
		}
		return outbound, host, nil
	}

	if strings.HasPrefix(raw, "vmess://") {
		b64 := strings.TrimPrefix(raw, "vmess://")
		decoded, err := decodeBase64(b64)
		if err != nil {
			return nil, "", err
		}

		var data struct {
			Add  string      `json:"add"`
			Port interface{} `json:"port"`
			ID   string      `json:"id"`
			Aid  interface{} `json:"aid"`
			Scy  string      `json:"scy"`
			Net  string      `json:"net"`
			Host string      `json:"host"`
			Path string      `json:"path"`
			TLS  string      `json:"tls"`
			SNI  string      `json:"sni"`
		}

		if err := json.Unmarshal([]byte(decoded), &data); err != nil {
			return nil, "", err
		}

		port := 443
		switch v := data.Port.(type) {
		case float64:
			port = int(v)
		case string:
			port, _ = strconv.Atoi(v)
		}

		security := data.Scy
		if security == "" {
			security = "auto"
		}

		aid := 0
		switch v := data.Aid.(type) {
		case float64:
			aid = int(v)
		case string:
			aid, _ = strconv.Atoi(v)
		}

		vnext := []map[string]interface{}{
			{
				"address": data.Add,
				"port":    port,
				"users": []map[string]interface{}{
					{
						"id":       data.ID,
						"alterId":  aid,
						"security": security,
					},
				},
			},
		}

		netType := data.Net
		if netType == "" {
			netType = "tcp"
		}

		streamSettings := map[string]interface{}{
			"network": netType,
		}
		if data.TLS == "tls" {
			streamSettings["security"] = "tls"
			sni := data.SNI
			if sni == "" {
				sni = data.Host
			}
			if sni == "" {
				sni = data.Add
			}
			streamSettings["tlsSettings"] = map[string]interface{}{
				"serverName":    sni,
				"allowInsecure": false,
				"fingerprint":   "chrome",
			}
		}

		path := data.Path
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		hostHeader := data.Host
		if hostHeader == "" {
			hostHeader = data.SNI
		}
		if hostHeader == "" {
			hostHeader = data.Add
		}

		if netType == "ws" {
			streamSettings["wsSettings"] = map[string]interface{}{
				"path": path,
				"headers": map[string]string{
					"Host": hostHeader,
				},
			}
		} else if netType == "grpc" {
			streamSettings["grpcSettings"] = map[string]interface{}{
				"serviceName": data.Path,
			}
		} else if netType == "httpupgrade" {
			streamSettings["httpupgradeSettings"] = map[string]interface{}{
				"path": path,
				"host": hostHeader,
			}
		}

		outbound := map[string]interface{}{
			"tag":      "proxy",
			"protocol": "vmess",
			"settings": map[string]interface{}{
				"vnext": vnext,
			},
			"streamSettings": streamSettings,
		}
		return outbound, data.Add, nil
	}

	if strings.HasPrefix(raw, "trojan://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, "", err
		}
		password := u.User.Username()
		host := u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		if port == 0 {
			port = 443
		}
		q := u.Query()
		sni := q.Get("sni")
		hostHeader := q.Get("host")
		if sni == "" && hostHeader != "" && !isIPAddress(hostHeader) {
			sni = hostHeader
		}
		if sni == "" && !isIPAddress(host) {
			sni = host
		}
		if hostHeader == "" {
			if sni != "" && !isIPAddress(sni) {
				hostHeader = sni
			} else if !isIPAddress(host) {
				hostHeader = host
			}
		}
		netType := q.Get("type")
		if netType == "" {
			netType = "tcp"
		}
		path := q.Get("path")
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		streamSettings := map[string]interface{}{
			"network":  netType,
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName":    sni,
				"allowInsecure": false,
				"fingerprint":   "chrome",
			},
		}
		if netType == "ws" {
			streamSettings["wsSettings"] = map[string]interface{}{
				"path": path,
				"headers": map[string]string{
					"Host": hostHeader,
				},
			}
		} else if netType == "grpc" {
			serviceName := q.Get("serviceName")
			if serviceName == "" {
				serviceName = q.Get("service_name")
			}
			streamSettings["grpcSettings"] = map[string]interface{}{
				"serviceName": serviceName,
			}
		}

		outbound := map[string]interface{}{
			"tag":      "proxy",
			"protocol": "trojan",
			"settings": map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"address":  host,
						"port":     port,
						"password": password,
					},
				},
			},
			"streamSettings": streamSettings,
		}
		return outbound, host, nil
	}

	if strings.HasPrefix(raw, "ss://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, "", err
		}
		host := u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		userInfo := u.User.String()
		method := "aes-256-gcm"
		password := ""
		if decoded, err := decodeBase64(userInfo); err == nil {
			parts := strings.SplitN(decoded, ":", 2)
			if len(parts) == 2 {
				method = parts[0]
				password = parts[1]
			}
		}

		outbound := map[string]interface{}{
			"tag":      "proxy",
			"protocol": "shadowsocks",
			"settings": map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"address":  host,
						"port":     port,
						"method":   method,
						"password": password,
						"ota":      false,
					},
				},
			},
		}
		return outbound, host, nil
	}

	return nil, "", fmt.Errorf("unsupported Xray URI scheme")
}

// BuildXrayConfigFromURI converts any VLESS/VMess/Trojan/SS URI into a complete native Xray-core JSON config
func BuildXrayConfigFromURI(rawURI string, httpPort int, socksPort int, routingMode string, settings config.Settings) (string, error) {
	if httpPort <= 0 {
		httpPort = 10809
	}
	if socksPort <= 0 {
		socksPort = 10808
	}
	if routingMode == "" {
		routingMode = "bypass_iran"
	}

	outbound, serverHost, err := ParseXrayOutboundFromURI(rawURI)
	if err != nil {
		return "", err
	}

	inbounds := []map[string]interface{}{
		{
			"tag":      "http-in",
			"port":     httpPort,
			"listen":   "127.0.0.1",
			"protocol": "http",
			"sniffing": map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
			},
			"settings": map[string]interface{}{
				"allowTransparent": false,
			},
		},
		{
			"tag":      "socks-in",
			"port":     socksPort,
			"listen":   "127.0.0.1",
			"protocol": "socks",
			"sniffing": map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls", "quic"},
			},
			"settings": map[string]interface{}{
				"auth": "noauth",
				"udp":  true,
			},
		},
	}

	if settings.FragmentEnabled {
		// The length/interval pair belongs to the extra "fragment" outbound
		// appended below; here the main outbound only has to be pointed at it.
		// (An earlier copy of the pair at this spot was never read.)
		if streamSettings, ok := outbound["streamSettings"].(map[string]interface{}); ok {
			streamSettings["sockopt"] = map[string]interface{}{
				"dialerProxy": "fragment",
			}
		}
	}

	outbounds := []map[string]interface{}{
		outbound,
		{
			"tag":      "direct",
			"protocol": "freedom",
			"settings": map[string]interface{}{
				"domainStrategy": "UseIP",
			},
		},
		{
			"tag":      "block",
			"protocol": "blackhole",
			"settings": map[string]interface{}{
				"response": map[string]string{
					"type": "none",
				},
			},
		},
	}

	if settings.FragmentEnabled {
		fragLength := settings.FragmentSize
		if fragLength == "" {
			fragLength = "100-200"
		}
		fragInterval := settings.FragmentSleep
		if fragInterval == "" {
			fragInterval = "10-20"
		}
		outbounds = append(outbounds, map[string]interface{}{
			"tag":      "fragment",
			"protocol": "freedom",
			"settings": map[string]interface{}{
				"domainStrategy": "AsIs",
				"fragment": map[string]interface{}{
					"packets":  "tlshello",
					"length":   fragLength,
					"interval": fragInterval,
				},
			},
		})
	}

	// Xray Routing Rules
	rules := []map[string]interface{}{
		{
			"type":        "field",
			"outboundTag": "direct",
			"ip":          []string{"geoip:private"},
		},
	}

	if serverHost != "" {
		if isIPAddress(serverHost) {
			rules = append(rules, map[string]interface{}{
				"type":        "field",
				"outboundTag": "direct",
				"ip":          []string{serverHost + "/32"},
			})
		} else {
			rules = append(rules, map[string]interface{}{
				"type":        "field",
				"outboundTag": "direct",
				"domain":      []string{serverHost},
			})
		}
	}

	// App split tunneling cannot be expressed in Xray at all: it has no
	// process-based routing, and the feature exists only in Sing-box TUN mode.
	// Both routing modes therefore add no rule here. The warning is the
	// caller's job — silently skipping is what made this look like a bug.

	if routingMode == "direct_iran_block_ads" {
		rules = append(rules, map[string]interface{}{
			"type":        "field",
			"outboundTag": "block",
			"domain":      []string{"geosite:category-ads-all", "keyword:doubleclick", "keyword:adservice", "keyword:tracker", "keyword:telemetry", "keyword:phishing", "keyword:malware"},
		})
	}

	if routingMode == "bypass_iran" || routingMode == "direct_iran_block_ads" || routingMode == "" {
		rules = append(rules,
			map[string]interface{}{
				"type":        "field",
				"outboundTag": "direct",
				"ip":          []string{"geoip:ir", "geoip:cn", "geoip:private"},
			},
			map[string]interface{}{
				"type":        "field",
				"outboundTag": "direct",
				"domain":      []string{"domain:ir", "regexp:^.*\\.ir$"},
			},
		)
	}

	// User-defined routing overrides.
	//
	// Xray, unlike sing-box, ends its rule list with an unconditional catch-all
	// that routes everything to the proxy. The overrides therefore have to be
	// inserted *before* that entry — appending them afterwards would leave them
	// unreachable, and the failure would be silent: the config is valid, the
	// core starts, and the rule simply never applies.
	rules = append(rules, UserRoutingRulesXray(settings.RoutingRules, "proxy")...)

	// Final default to proxy
	rules = append(rules, map[string]interface{}{
		"type":        "field",
		"outboundTag": "proxy",
		"network":     "tcp,udp",
	})

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"dns": map[string]interface{}{
			"servers": []interface{}{
				"1.1.1.1",
				"8.8.8.8",
				map[string]interface{}{
					"address":   "178.22.122.100",
					"port":      53,
					"domains":   []string{"domain:ir", "regexp:^.*\\.ir$"},
					"expectIPs": []string{"geoip:ir"},
				},
				"localhost",
			},
			"queryStrategy": "UseIPv4",
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing": map[string]interface{}{
			"domainStrategy": "IPIfNonMatch",
			"rules":          rules,
		},
	}

	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
