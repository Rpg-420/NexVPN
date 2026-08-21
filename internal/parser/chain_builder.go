package parser

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nexvpn/internal/config"
)

// BuildMultiHopChain constructs a unified Sing-box nested tunnel chain from ordered profiles
func BuildMultiHopChain(hops []config.Profile, chainName string, settings config.Settings) (*config.Profile, error) {
	if len(hops) < 2 {
		return nil, fmt.Errorf("multi-hop chaining requires at least 2 profiles")
	}

	var parsedOutbounds []map[string]interface{}
	var hopNames []string

	for i, hop := range hops {
		var outbound map[string]interface{}
		var err error

		if hop.RawURI != "" {
			node, parseErr := ParseURI(hop.RawURI)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse hop %d (%s): %w", i+1, hop.Name, parseErr)
			}
			outbound = node.Outbound
		} else if strings.Contains(hop.ConfigContent, "[Interface]") {
			outbound, _, err = ParseWireGuardConf(hop.ConfigContent)
			if err != nil {
				return nil, fmt.Errorf("failed to parse WireGuard hop %d (%s): %w", i+1, hop.Name, err)
			}
		} else if hop.ConfigContent != "" {
			// Try extracting outbound from Singbox JSON
			var sbCfg struct {
				Outbounds []map[string]interface{} `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(hop.ConfigContent), &sbCfg); err == nil && len(sbCfg.Outbounds) > 0 {
				outbound = sbCfg.Outbounds[0]
			}
		}

		if outbound == nil {
			return nil, fmt.Errorf("hop %d (%s) has no valid outbound configuration", i+1, hop.Name)
		}

		parsedOutbounds = append(parsedOutbounds, outbound)
		hopNames = append(hopNames, hop.Name)
	}

	// Chain the outbounds together via "detour"
	// Hop 0: Ingress/Bridge (no detour)
	// Hop 1: detour -> tag(Hop 0)
	// Hop 2: detour -> tag(Hop 1)
	// Last Hop: tag -> "proxy", detour -> tag(Hop N-1)
	var chainedOutbounds []map[string]interface{}
	totalHops := len(parsedOutbounds)

	for i := 0; i < totalHops; i++ {
		out := parsedOutbounds[i]
		if i == 0 {
			out["tag"] = "chain-hop-1"
			delete(out, "detour")
		} else if i == totalHops-1 {
			out["tag"] = "proxy"
			out["detour"] = fmt.Sprintf("chain-hop-%d", i)
		} else {
			out["tag"] = fmt.Sprintf("chain-hop-%d", i+1)
			out["detour"] = fmt.Sprintf("chain-hop-%d", i)
		}
		chainedOutbounds = append(chainedOutbounds, out)
	}

	// Inbounds
	tunStack := "gvisor"
	tunMTU := 1500
	if settings.GameModeEnabled {
		tunStack = "mixed"
		tunMTU = 1400
	}

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
			"inet6_address":              "fdfe:dcba:9876::1/126",
			"auto_route":                 true,
			"strict_route":               false,
			"stack":                      tunStack,
			"mtu":                        tunMTU,
			"sniff":                      true,
			"sniff_override_destination": false,
			"endpoint_independent_nat":   true,
		},
	}

	// Standard outbounds
	outbounds := append(chainedOutbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "dns", "tag": "dns-out"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)

	// Routing rules
	routeRules := []map[string]interface{}{
		{"protocol": []string{"dns"}, "outbound": "dns-out"},
		{"ip_cidr": []string{"192.168.0.0/16", "127.0.0.0/8", "169.254.0.0/16"}, "outbound": "direct"},
	}

	if settings.RoutingMode == "bypass_iran" || settings.RoutingMode == "direct_iran_block_ads" || settings.RoutingMode == "" {
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

	if settings.BlockAds || settings.RoutingMode == "direct_iran_block_ads" {
		routeRules = append(routeRules, map[string]interface{}{
			"domain_keyword": []string{"doubleclick", "adservice", "googleadservices", "adnxs", "adroll", "analytics", "applovin", "unityads", "vungle", "admob"},
			"outbound":       "block",
		})
	}

	fullConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "warn",
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{"tag": "dns-direct", "address": "local", "detour": "direct"},
				{"tag": "dns-remote", "address": "udp://1.1.1.1", "address_resolver": "dns-direct", "detour": "proxy"},
			},
			"rules": []map[string]interface{}{
				{"domain_suffix": []string{".ir"}, "server": "dns-direct"},
				{"outbound": "direct", "server": "dns-direct"},
			},
			"final":    "dns-remote",
			"strategy": "ipv4_only",
		},
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"final":                 "proxy",
			"rules":                 routeRules,
		},
	}

	jsonBytes, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return nil, err
	}

	if chainName == "" {
		chainName = fmt.Sprintf("Proxy Chain: %s", strings.Join(hopNames, " ➔ "))
	}

	profile := &config.Profile{
		Name:          chainName,
		Protocol:      config.ProtocolSingBox,
		Category:      config.CategoryV2Ray,
		Group:         "Proxy Chains",
		ConfigContent: string(jsonBytes),
		Description:   fmt.Sprintf("Nested Multi-Hop Tunnel: %s", strings.Join(hopNames, " ➔ ")),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	return profile, nil
}
