package parser

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nexvpn/internal/config"
)

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

// BuildSingBoxConfig constructs standard Sing-box configuration
func BuildSingBoxConfig(proxyOutbound map[string]interface{}, proxyMode string, routingMode string, serverHost string) (string, error) {
	return BuildSingBoxConfigAdvanced(proxyOutbound, proxyMode, routingMode, serverHost, config.Settings{
		ProxyMode:   proxyMode,
		RoutingMode: routingMode,
	})
}

// BuildSingBoxConfigAdvanced constructs a state-of-the-art Sing-box 1.9+ config with Anti-DPI, App Split Tunneling, and Multi-Hop
func BuildSingBoxConfigAdvanced(proxyOutbound map[string]interface{}, proxyMode string, routingMode string, serverHost string, settings config.Settings) (string, error) {
	// Deep copy outbound so mutations don't affect parent
	outboundBytes, _ := json.Marshal(proxyOutbound)
	var activeOutbound map[string]interface{}
	_ = json.Unmarshal(outboundBytes, &activeOutbound)

	// Ensure WireGuard outbounds never contain TLS fields
	if outType, ok := activeOutbound["type"].(string); ok && outType == "wireguard" {
		delete(activeOutbound, "tls")
		delete(activeOutbound, "tls_tricks")
		if settings.AmneziaJc > 0 {
			activeOutbound["junk_packet_count"] = settings.AmneziaJc
			activeOutbound["junk_packet_min_size"] = settings.AmneziaJmin
			activeOutbound["junk_packet_max_size"] = settings.AmneziaJmax
			activeOutbound["init_packet_junk_size"] = settings.AmneziaS1
			activeOutbound["response_packet_junk_size"] = settings.AmneziaS2
			activeOutbound["init_packet_magic_header"] = settings.AmneziaH1
			activeOutbound["response_packet_magic_header"] = settings.AmneziaH2
			activeOutbound["underload_packet_magic_header"] = settings.AmneziaH3
			activeOutbound["transport_header_magic_header"] = settings.AmneziaH4
		}
	}

	listenAddr := "127.0.0.1"
	if settings.AllowLan {
		listenAddr = "0.0.0.0"
	}

	// Inbounds: Mixed on 10808 (SOCKS5/HTTP) AND Dedicated HTTP on 10809 (For Windows System Proxy)
	inbounds := []map[string]interface{}{
		{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      listenAddr,
			"listen_port": 10808,
			"sniff":       true,
		},
		{
			"type":        "http",
			"tag":         "http-in",
			"listen":      listenAddr,
			"listen_port": 10809,
			"sniff":       true,
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
			"type":                       "tun",
			"tag":                        "tun-in",
			"interface_name":             "NexVPN-TUN",
			"inet4_address":              "172.19.0.1/30",
			"auto_route":                 true,
			"strict_route":               false,
			"stack":                      tunStack,
			"mtu":                        tunMTU,
			"sniff":                      true,
			"sniff_override_destination": false,
		}
		inbounds = append(inbounds, tunInbound)
	}

	primaryTag := "proxy"
	outbounds := []map[string]interface{}{
		activeOutbound,
	}

	// Multi-Hop WARP Chain: local -> VPS proxy -> WARP -> target
	// Fix #8: Register a fresh WARP key pair from Cloudflare API instead of using hardcoded keys
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

		warpOutbound := map[string]interface{}{
			"type":             "wireguard",
			"tag":              "warp-chain",
			"server":           warpCleanIP,
			"server_port":      warpPort,
			"system_interface": false,
			"local_address":    []string{warpLocalAddr},
			"private_key":      warpPrivKey,
			"peer_public_key":  warpPubKey,
			"mtu":              1280,
			"detour":           "proxy",
		}
		if len(warpReserved) > 0 {
			warpOutbound["reserved"] = warpReserved
		}
		outbounds = append(outbounds, warpOutbound)
		primaryTag = "warp-chain"
	}


	outbounds = append(outbounds,
		map[string]interface{}{
			"type": "direct",
			"tag":  "direct",
		},
		map[string]interface{}{
			"type": "dns",
			"tag":  "dns-out",
		},
		map[string]interface{}{
			"type": "block",
			"tag":  "block",
		},
	)

	// Smart Zero-Pollution Dual DNS Setup
	primaryDNSAddr := "tcp://1.1.1.1"
	if settings.DNSServer != "" {
		primaryDNSAddr = settings.DNSServer
	}

	fallbackDNSAddr := "tcp://8.8.8.8"
	if settings.DNSFallback != "" {
		fallbackDNSAddr = settings.DNSFallback
	}

	dnsServers := []map[string]interface{}{
		{
			"tag":     "dns-direct",
			"address": "tcp://8.8.8.8",
			"detour":  "direct",
		},
		{
			"tag":              "dns-remote",
			"address":          primaryDNSAddr,
			"detour":           primaryTag,
			"address_resolver": "dns-direct",
		},
		{
			"tag":              "dns-fallback",
			"address":          fallbackDNSAddr,
			"detour":           primaryTag,
			"address_resolver": "dns-direct",
		},
		{
			"tag":     "dns-block",
			"address": "rcode://success",
		},
	}

	// DNS Rules
	dnsRules := []map[string]interface{}{
		{
			"outbound": "direct",
			"server":   "dns-direct",
		},
	}

	if serverHost != "" && !isIPAddress(serverHost) {
		dnsRules = append(dnsRules, map[string]interface{}{
			"domain": []string{serverHost},
			"server": "dns-direct",
		})
	}

	if routingMode == "bypass_iran" || routingMode == "direct_iran_block_ads" || routingMode == "" {
		dnsRules = append(dnsRules, map[string]interface{}{
			"domain_suffix": []string{".ir"},
			"server":        "dns-direct",
		})
	}

	// Routing Rules
	routeRules := []map[string]interface{}{
		{
			"port":     []int{53},
			"outbound": "dns-out",
		},
		{
			"protocol": []string{"dns"},
			"outbound": "dns-out",
		},
		{
			// Fix #11: Include 172.16.0.0/12 in private range — was missing before
			"ip_cidr":  []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"},
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

	// Per-App Process Routing (Split Tunneling)
	if settings.AppRoutingMode == "bypass_selected" && len(settings.SelectedApps) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"process_name": settings.SelectedApps,
			"outbound":     "direct",
		})
	} else if settings.AppRoutingMode == "proxy_selected" && len(settings.SelectedApps) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"process_name": settings.SelectedApps,
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

	// Ad-blocking rules
	if settings.BlockAds || routingMode == "direct_iran_block_ads" {
		routeRules = append(routeRules, map[string]interface{}{
			"domain_keyword": []string{"doubleclick", "adservice", "googleadservices", "adnxs", "adroll", "analytics", "applovin", "unityads", "vungle", "admob"},
			"outbound":       "block",
		})
	}

	// Custom Blocked Domains
	if len(settings.CustomBlockDomains) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"domain":   settings.CustomBlockDomains,
			"outbound": "block",
		})
	}

	// Custom Direct Domains
	if len(settings.CustomDirectDomains) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"domain":   settings.CustomDirectDomains,
			"outbound": "direct",
		})
	}

	// Custom Proxied Domains
	if len(settings.CustomProxyDomains) > 0 {
		routeRules = append(routeRules, map[string]interface{}{
			"domain":   settings.CustomProxyDomains,
			"outbound": primaryTag,
		})
	}

	// Block unroutable IPv6 traffic when IPv4-only to eliminate no route to host errors
	if settings.IPStrategy != "prefer_ipv6" && settings.IPStrategy != "ipv6_only" {
		routeRules = append(routeRules, map[string]interface{}{
			"ip_cidr":  []string{"::/0"},
			"outbound": "block",
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
			"servers":           dnsServers,
			"rules":             dnsRules,
			"final":             "dns-remote",
			"strategy":          ipStrategy,
			"independent_cache": true,
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route": map[string]interface{}{
			"rules":                 routeRules,
			"final":                 primaryTag,
			"auto_detect_interface": true,
		},
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
		transportObj := map[string]interface{}{
			"type": "httpupgrade",
			"path": path,
			"host": hostHeader,
			"headers": map[string]string{
				"Host": hostHeader,
			},
		}
		outbound["transport"] = transportObj
	} else if netType == "http" {
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
		if sni == "" {
			sni = data.Host
		}
		if sni == "" {
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
		sni = host
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
	if ip == "" {
		ip = "172.16.0.2/32"
	}

	outbound := map[string]interface{}{
		"type":             "wireguard",
		"tag":              "proxy",
		"server":           host,
		"server_port":      port,
		"system_interface": false,
		"local_address":    []string{ip},
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
	}

	return outbound, nil
}

// ParseWireGuardConf parses a standard WireGuard INI config file (.conf)
func ParseWireGuardConf(confContent string) (map[string]interface{}, string, error) {
	var privKey, address, pubKey, endpoint string
	var mtu = 1280
	var reserved []int

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
		Protocol:      config.ProtocolSingBox,
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
		Protocol:      config.ProtocolSingBox,
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
	if cleanIP == "" {
		cleanIP = "162.159.192.1"
	}
	if port <= 0 {
		port = 2408
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
	req, err := http.NewRequest("POST", "https://api.cloudflareclient.com/v0a3250/reg", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	var v4Addr = "172.16.0.2/32"
	var peerPubKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
	var reserved []int

	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
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
			if len(resData.Config.Peers) > 0 && resData.Config.Peers[0].PublicKey != "" {
				peerPubKey = resData.Config.Peers[0].PublicKey
			}
		}
	}

	outbound := map[string]interface{}{
		"type":             "wireguard",
		"tag":              "proxy",
		"server":           cleanIP,
		"server_port":      port,
		"system_interface": false,
		"local_address":    []string{v4Addr},
		"private_key":      privB64,
		"peer_public_key":  peerPubKey,
		"mtu":              1280,
	}

	if len(reserved) > 0 {
		outbound["reserved"] = reserved
	}

	singboxJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", cleanIP)
	if err != nil {
		return nil, err
	}

	reservedParam := ""
	if len(reserved) > 0 {
		reservedParam = fmt.Sprintf("&reserved=%d,%d,%d", reserved[0], reserved[1], reserved[2])
	}

	profile := &config.Profile{
		Name:          fmt.Sprintf("Cloudflare WARP (%s)", cleanIP),
		Protocol:      config.ProtocolWARP,
		Category:      config.CategoryWireGuardWarp,
		Server:        cleanIP,
		Port:          port,
		Group:         "Cloudflare WARP",
		ConfigContent: singboxJSON,
		RawURI:        fmt.Sprintf("wireguard://%s@%s:%d?public_key=%s&ip=%s%s#WARP", url.QueryEscape(privB64), cleanIP, port, url.QueryEscape(peerPubKey), url.QueryEscape(v4Addr), reservedParam),
		Description:   "Fast, unlimited Cloudflare WARP tunnel with Clean IP",
		WarpCleanIP:   cleanIP,
		CreatedAt:     time.Now(),
	}

	return profile, nil
}

// GenerateWarpOnWarp creates a nested dual-hop WARP-on-WARP configuration
func GenerateWarpOnWarp(cleanIP string, port int, egressIP string, egressPort int) (*config.Profile, error) {
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

	var hop1Outbound, hop2Outbound map[string]interface{}
	var hop1Config, hop2Config struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(hop1.ConfigContent), &hop1Config); err == nil && len(hop1Config.Outbounds) > 0 {
		hop1Outbound = hop1Config.Outbounds[0]
	}
	if err := json.Unmarshal([]byte(hop2.ConfigContent), &hop2Config); err == nil && len(hop2Config.Outbounds) > 0 {
		hop2Outbound = hop2Config.Outbounds[0]
	}

	if hop1Outbound == nil || hop2Outbound == nil {
		return nil, fmt.Errorf("failed to extract outbound configurations for WARP on WARP")
	}

	hop1Outbound["tag"] = "warp-hop1"
	delete(hop1Outbound, "detour")

	hop2Outbound["tag"] = "proxy"
	hop2Outbound["detour"] = "warp-hop1"

	// Build custom nested config
	inbounds := []map[string]interface{}{
		{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      "127.0.0.1",
			"listen_port": 10808,
			"sniff":       true,
		},
		{
			"type":        "http",
			"tag":         "http-in",
			"listen":      "127.0.0.1",
			"listen_port": 10809,
			"sniff":       true,
		},
		{
			"type":                       "tun",
			"tag":                        "tun-in",
			"interface_name":             "NexVPN-TUN",
			"inet4_address":              "172.19.0.1/30",
			"auto_route":                 true,
			"strict_route":               false,
			"stack":                      "gvisor",
			"sniff":                      true,
			"endpoint_independent_nat":   true,
		},
	}

	outbounds := []map[string]interface{}{
		hop2Outbound,
		hop1Outbound,
		{"type": "direct", "tag": "direct"},
		{"type": "dns", "tag": "dns-out"},
		{"type": "block", "tag": "block"},
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{"level": "warn"},
		"inbounds": inbounds,
		"outbounds": outbounds,
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{"tag": "dns-direct", "address": "local", "detour": "direct"},
				{"tag": "dns-remote", "address": "tcp://1.1.1.1", "detour": "proxy"},
			},
			"rules": []map[string]interface{}{
				{"outbound": "direct", "server": "dns-direct"},
			},
			"final": "dns-remote",
			"strategy": "ipv4_only",
		},
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"final": "proxy",
			"rules": []map[string]interface{}{
				{"protocol": []string{"dns"}, "outbound": "dns-out"},
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
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("no valid nodes found in subscription")
	}

	return nodes, nil
}

// TCPPing performs a fast TCP connectivity test to server:port
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
	defer conn.Close()

	return int(time.Since(start).Milliseconds())
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
	defer resp.Body.Close()

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
			conn.Close()
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
			defer resp.Body.Close()
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
			defer resp.Body.Close()
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
			defer resp.Body.Close()
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

func isIPAddress(host string) bool {
	return net.ParseIP(host) != nil
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
			streamSettings["realitySettings"] = map[string]interface{}{
				"serverName":  sni,
				"fingerprint": fp,
				"publicKey":   cleanPbk,
				"shortId":     sid,
				"spiderX":     "",
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
			streamSettings["splithttpSettings"] = map[string]interface{}{
				"path": path,
				"host": hostHeader,
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
		sni := u.Query().Get("sni")
		if sni == "" {
			sni = host
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
			"streamSettings": map[string]interface{}{
				"security": "tls",
				"tlsSettings": map[string]interface{}{
					"serverName":    sni,
					"allowInsecure": false,
					"fingerprint":   "chrome",
				},
			},
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

	if settings.AppRoutingMode == "bypass_selected" && len(settings.SelectedApps) > 0 {
		// Note: Xray does not support process-based routing. This rule has no effect.
		// App split tunneling is only available in Sing-box TUN mode.
		// Silently skip — a warning will be logged by the caller.
	} else if settings.AppRoutingMode == "proxy_selected" && len(settings.SelectedApps) > 0 {
		// Note: Xray does not support process-based routing.
		// App split tunneling is only available in Sing-box TUN mode.
	}

	if routingMode == "direct_iran_block_ads" {
		rules = append(rules, map[string]interface{}{
			"type":        "field",
			"outboundTag": "block",
			"domain":      []string{"geosite:category-ads-all"},
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
