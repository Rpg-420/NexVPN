package core

import (
	"encoding/json"
	"nexvpn/internal/config"
)

// SingBoxConfigBuilder provides a type-safe, fluent pipeline for generating Sing-box configs
type SingBoxConfigBuilder struct {
	inbounds  []map[string]interface{}
	outbounds []map[string]interface{}
	dns       map[string]interface{}
	route     map[string]interface{}
	logLevel  string
}

// NewConfigBuilder initializes a fresh builder instance with zero-leak defaults
func NewConfigBuilder() *SingBoxConfigBuilder {
	return &SingBoxConfigBuilder{
		inbounds:  make([]map[string]interface{}, 0, 4),
		outbounds: make([]map[string]interface{}, 0, 4),
		logLevel:  "warn",
	}
}

// WithStandardInbounds configures mixed SOCKS5/HTTP and high-performance gVisor TUN
func (b *SingBoxConfigBuilder) WithStandardInbounds(proxyMode string, gameModeEnabled bool) *SingBoxConfigBuilder {
	// Mixed & HTTP Inbounds
	b.inbounds = append(b.inbounds,
		map[string]interface{}{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      "127.0.0.1",
			"listen_port": 10808,
			"sniff":       true,
		},
		map[string]interface{}{
			"type":        "http",
			"tag":         "http-in",
			"listen":      "127.0.0.1",
			"listen_port": 10809,
			"sniff":       true,
		},
	)

	// High-performance TUN Inbound — sync'd with parser.go
	if proxyMode == "tun" || proxyMode == "" {
		tunStack := "gvisor"
		tunMTU := 1500
		if gameModeEnabled {
			tunStack = "mixed"
			tunMTU = 1400
		}
		b.inbounds = append(b.inbounds, map[string]interface{}{
			"type":                       "tun",
			"tag":                        "tun-in",
			"interface_name":             "NexVPN-TUN",
			"inet4_address":              []string{"172.19.0.1/30"},
			"inet6_address":              []string{"fdfe:dcba:9876::1/126"},
			"auto_route":                 true,
			"strict_route":               false,
			"stack":                      tunStack,
			"mtu":                        tunMTU,
			"sniff":                      true,
			"sniff_override_destination": false,
			"endpoint_independent_nat":   true,
		})
	}
	return b
}

// WithProxyOutbound attaches the primary anti-censorship outbound and standard helpers
func (b *SingBoxConfigBuilder) WithProxyOutbound(activeOutbound map[string]interface{}) *SingBoxConfigBuilder {
	b.outbounds = append(b.outbounds,
		activeOutbound,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "dns", "tag": "dns-out"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)
	return b
}

// WithSmartDNS configures non-poisoned dual DNS with automatic direct bootstrap
func (b *SingBoxConfigBuilder) WithSmartDNS(primaryDNS, fallbackDNS, ipStrategy string) *SingBoxConfigBuilder {
	if primaryDNS == "" {
		primaryDNS = "1.1.1.1"
	}
	if fallbackDNS == "" {
		fallbackDNS = "8.8.8.8"
	}
	if ipStrategy == "" {
		ipStrategy = "ipv4_only"
	}

	b.dns = map[string]interface{}{
		"servers": []map[string]interface{}{
			{"tag": "dns-direct", "address": "1.1.1.1", "detour": "direct"},
			{"tag": "dns-remote", "address": primaryDNS, "detour": "proxy"},
			{"tag": "dns-fallback", "address": fallbackDNS, "detour": "proxy"},
			{"tag": "dns-block", "address": "rcode://success"},
		},
		"rules": []map[string]interface{}{
			{"outbound": "direct", "server": "dns-direct"},
			{"domain_suffix": []string{".ir"}, "server": "dns-direct"},
		},
		"final":             "dns-remote",
		"strategy":          ipStrategy,
		"independent_cache": true,
	}
	return b
}

// WithRoutingRules applies domain bypass, app routing, and unroutable IPv6 drops
func (b *SingBoxConfigBuilder) WithRoutingRules(settings config.Settings) *SingBoxConfigBuilder {
	rules := []map[string]interface{}{
		{"protocol": []string{"dns"}, "outbound": "dns-out"},
		{"ip_cidr": []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}, "outbound": "direct"},
	}

	// Custom user block domains
	if len(settings.CustomBlockDomains) > 0 {
		rules = append(rules, map[string]interface{}{"domain_keyword": settings.CustomBlockDomains, "outbound": "block"})
	}

	// Custom user direct domains
	if len(settings.CustomDirectDomains) > 0 {
		rules = append(rules, map[string]interface{}{"domain_keyword": settings.CustomDirectDomains, "outbound": "direct"})
	}

	// Custom user proxy domains
	if len(settings.CustomProxyDomains) > 0 {
		rules = append(rules, map[string]interface{}{"domain_keyword": settings.CustomProxyDomains, "outbound": "proxy"})
	}

	// Domestic Iran & Zero-Disconnect Banking bypass
	if settings.RoutingMode != "global" {
		rules = append(rules,
			map[string]interface{}{"domain_suffix": []string{".ir"}, "outbound": "direct"},
			map[string]interface{}{"domain_keyword": []string{
				"telewebion", "digikala", "snapp", "shaparak", "divar", "torob", "aparat",
				"varzesh3", "bank", "bmi.ir", "sep.ir", "asanpardakht", "zarinpal",
				"sadadpsp", "pec.shaparak", "behpardakht", "irankish", "pasargad",
				"mellat", "tejarat", "saman", "melli", "parsian", "nobitex", "wallex",
				"bazaar", "tamin.ir", "sanjesh.org", "medu.ir", "epolice.ir",
			}, "outbound": "direct"},
			map[string]interface{}{"geoip": []string{"ir"}, "outbound": "direct"},
		)
	}

	// Visual Split-Tunneling App rules
	if settings.AppRoutingMode == "bypass_selected" && len(settings.SelectedApps) > 0 {
		rules = append(rules, map[string]interface{}{"process_name": settings.SelectedApps, "outbound": "direct"})
	} else if settings.AppRoutingMode == "proxy_selected" && len(settings.SelectedApps) > 0 {
		rules = append(rules, map[string]interface{}{"process_name": settings.SelectedApps, "outbound": "proxy"})
		rules = append(rules, map[string]interface{}{"outbound": "direct"})
	}

	// Block IPv6 to prevent no-route-to-host hangs
	if settings.IPStrategy != "prefer_ipv6" && settings.IPStrategy != "ipv6_only" {
		rules = append(rules, map[string]interface{}{"ip_cidr": []string{"::/0"}, "outbound": "block"})
	}

	b.route = map[string]interface{}{
		"rules":                 rules,
		"final":                 "proxy",
		"auto_detect_interface": true,
	}
	return b
}

// Build compiles the full configuration JSON
func (b *SingBoxConfigBuilder) Build() (string, error) {
	configMap := map[string]interface{}{
		"log":          map[string]interface{}{"level": b.logLevel},
		"experimental": map[string]interface{}{"clash_api": map[string]interface{}{"external_controller": "127.0.0.1:9090"}},
		"inbounds":     b.inbounds,
		"outbounds":    b.outbounds,
		"dns":          b.dns,
		"route":        b.route,
	}
	bytes, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
