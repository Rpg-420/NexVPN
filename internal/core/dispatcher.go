package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nexvpn/internal/config"
	"nexvpn/internal/parser"
	"nexvpn/internal/protocols"
)

// DispatchDecision holds the selected core, adapter, and the generated configuration
type DispatchDecision struct {
	CoreProtocol  config.Protocol
	ConfigContent string
	Adapter       protocols.ProtocolAdapter
	CoreName      string
	Description   string
}

// SmartCoreDispatcher analyzes profile and environment to select the optimal core and network routing
type SmartCoreDispatcher struct{}

// NewSmartCoreDispatcher creates a new dispatcher instance
func NewSmartCoreDispatcher() *SmartCoreDispatcher {
	return &SmartCoreDispatcher{}
}

// Decide selects the best available engine and builds its dedicated configuration
func (d *SmartCoreDispatcher) Decide(profile *config.Profile, settings config.Settings) (*DispatchDecision, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile is nil")
	}

	proxyMode := settings.ProxyMode
	if proxyMode == "" {
		proxyMode = "tun"
	}
	routingMode := settings.RoutingMode
	if routingMode == "" {
		routingMode = "bypass_iran"
	}

	// Aether Profile
	if profile.Protocol == config.ProtocolAether {
		adapter, err := protocols.NewAdapter(config.ProtocolAether)
		if err != nil {
			return nil, fmt.Errorf("aether adapter error: %w", err)
		}

		wrap := map[string]interface{}{
			"aether_config": profile.ConfigContent,
			"settings":      settings,
			"proxy_mode":    proxyMode,
			"routing_mode":  routingMode,
		}
		wrapBytes, _ := json.Marshal(wrap)

		return &DispatchDecision{
			CoreProtocol:  config.ProtocolAether,
			ConfigContent: string(wrapBytes),
			Adapter:       adapter,
			CoreName:      "Aether Core",
			Description:   "Aether user-space censorship circumvention engine (MASQUE / gool)",
		}, nil
	}

	// 1. OpenVPN Profile
	if profile.Protocol == config.ProtocolOpenVPN || profile.Category == config.CategoryOpenVPN {
		adapter, err := protocols.NewAdapter(config.ProtocolOpenVPN)
		if err != nil {
			return nil, fmt.Errorf("openvpn adapter error: %w", err)
		}
		if ovpn, ok := adapter.(*protocols.OpenVPNAdapter); ok {
			ovpn.SetCredentials(profile.Username, profile.Password)
		}
		return &DispatchDecision{
			CoreProtocol:  config.ProtocolOpenVPN,
			ConfigContent: profile.ConfigContent,
			Adapter:       adapter,
			CoreName:      "OpenVPN Core",
			Description:   "Native OpenVPN client with TAP/Wintun adapter routing",
		}, nil
	}

	// 2. WireGuard / WARP .conf Profile
	if (profile.Protocol == config.ProtocolWireGuard || profile.Protocol == config.ProtocolWARP || profile.Category == config.CategoryWireGuardWarp) && strings.Contains(profile.ConfigContent, "[Interface]") {
		outbound, host, err := parser.ParseWireGuardConf(profile.ConfigContent)
		if err != nil {
			return nil, fmt.Errorf("wireguard config parse error: %w", err)
		}
		if outbound == nil {
			return nil, fmt.Errorf("wireguard config produced empty outbound")
		}
		sbJSON, err := parser.BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, host, settings)
		if err != nil {
			return nil, fmt.Errorf("wireguard singbox config build error: %w", err)
		}
		adapter, _ := protocols.NewAdapter(config.ProtocolSingBox)
		return &DispatchDecision{
			CoreProtocol:  config.ProtocolSingBox,
			ConfigContent: sbJSON,
			Adapter:       adapter,
			CoreName:      "Sing-box (WireGuard Engine)",
			Description:   "High-performance Sing-box WireGuard kernel with multi-queue TUN routing",
		}, nil
	}

	// 3. Profiles with RawURI (VLESS, VMess, Trojan, SS, Hysteria 2, TUIC, WireGuard, WARP)
	if profile.RawURI != "" {
		uri := strings.TrimSpace(profile.RawURI)

		// UDP-heavy protocols, Hysteria 2, TUIC, and WARP/WireGuard are natively optimized in Sing-Box
		isSingBoxPreferred := strings.HasPrefix(uri, "hy2://") ||
			strings.HasPrefix(uri, "hysteria2://") ||
			strings.HasPrefix(uri, "tuic://") ||
			strings.HasPrefix(uri, "wireguard://") ||
			strings.HasPrefix(uri, "wg://") ||
			profile.Protocol == config.ProtocolWARP ||
			profile.Protocol == config.ProtocolHysteria2

		// User Core Preference: "xray", "singbox", or "auto"
		v2rayCorePref := settings.V2RayCore

		// 1. If TUN mode is requested, or Sing-box preferred protocols (Hysteria 2, TUIC, Wireguard), use Sing-Box for true Layer-3 Wintun TUN routing
		if proxyMode == "tun" || isSingBoxPreferred || v2rayCorePref == "singbox" {
			outbound, err := parser.ParseOutboundFromURI(uri)
			if err == nil && outbound != nil {
				host := profile.Server
				if host == "" {
					if s, ok := outbound["server"].(string); ok {
						host = s
					}
				}
				sbJSON, err := parser.BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, host, settings)
				if err == nil {
					adapter, _ := protocols.NewAdapter(config.ProtocolSingBox)
					return &DispatchDecision{
						CoreProtocol:  config.ProtocolSingBox,
						ConfigContent: sbJSON,
						Adapter:       adapter,
						CoreName:      "Sing-box Core (Full-System TUN Engine)",
						Description:   "Universal Sing-box engine with Wintun virtual network adapter and anti-DPI routing",
					}, nil
				}
			}
		}

		// 2. If System Proxy mode and not forced to singbox, use Xray-Core
		if v2rayCorePref != "singbox" {
			xrayJSON, err := parser.BuildXrayConfigFromURI(uri, 10809, 10808, routingMode, settings)
			if err == nil && xrayJSON != "" {
				adapter, err := protocols.NewAdapter(config.ProtocolXray)
				if err == nil {
					return &DispatchDecision{
						CoreProtocol:  config.ProtocolXray,
						ConfigContent: xrayJSON,
						Adapter:       adapter,
						CoreName:      "Xray-Core (V2Ray / Reality Engine)",
						Description:   "Native Xray-core with XTLS Reality, VLESS, VMess, and zero-pollution GeoIP routing",
					}, nil
				}
			}
		}

		// 3. Fallback: Parse into Sing-box
		outbound, err := parser.ParseOutboundFromURI(uri)
		if err == nil && outbound != nil {
			host := profile.Server
			if host == "" {
				if s, ok := outbound["server"].(string); ok {
					host = s
				}
			}
			sbJSON, err := parser.BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, host, settings)
			if err == nil {
				adapter, _ := protocols.NewAdapter(config.ProtocolSingBox)
				return &DispatchDecision{
					CoreProtocol:  config.ProtocolSingBox,
					ConfigContent: sbJSON,
					Adapter:       adapter,
					CoreName:      "Sing-box Core (TUN/UDP Engine)",
					Description:   "Universal Sing-box engine with Wintun virtual network adapter and anti-DPI routing",
				}, nil
			}
		}
	}

	// 4. Raw JSON Content (Custom Sing-Box or Xray JSON)
	if profile.ConfigContent != "" {
		trimmed := strings.TrimSpace(profile.ConfigContent)
		if strings.HasPrefix(trimmed, "{") {
			isXray := strings.Contains(trimmed, `"inbounds"`) &&
				strings.Contains(trimmed, `"outbounds"`) &&
				strings.Contains(trimmed, `"routing"`)

			if isXray {
				adapter, _ := protocols.NewAdapter(config.ProtocolXray)
				return &DispatchDecision{
					CoreProtocol:  config.ProtocolXray,
					ConfigContent: profile.ConfigContent,
					Adapter:       adapter,
					CoreName:      "Xray-Core (Custom JSON)",
					Description:   "Custom Xray-core configuration",
				}, nil
			}

			adapter, _ := protocols.NewAdapter(config.ProtocolSingBox)
			return &DispatchDecision{
				CoreProtocol:  config.ProtocolSingBox,
				ConfigContent: profile.ConfigContent,
				Adapter:       adapter,
				CoreName:      "Sing-box Core (Custom JSON)",
				Description:   "Custom Sing-box JSON configuration",
			}, nil
		}
	}

	return nil, fmt.Errorf("unable to determine core or generate valid configuration for profile: %s", profile.Name)
}

// EnsureWintunDriver checks and ensures wintun.dll is present in working directories
func EnsureWintunDriver() {
	execDir, err := os.Executable()
	if err != nil {
		return
	}
	execFolder := filepath.Dir(execDir)
	binDir := filepath.Join(execFolder, "bin")

	// Source candidates
	candidates := []string{
		filepath.Join(binDir, "wintun.dll"),
		filepath.Join(execFolder, "wintun.dll"),
		"wintun.dll",
		"bin/wintun.dll",
	}

	var validSource string
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 10000 {
			validSource = c
			break
		}
	}

	if validSource == "" {
		return
	}

	// Ensure present in bin folder and execFolder
	targets := []string{
		filepath.Join(execFolder, "wintun.dll"),
		filepath.Join(binDir, "wintun.dll"),
	}

	srcBytes, err := os.ReadFile(validSource)
	if err != nil {
		return
	}

	for _, target := range targets {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			_ = os.WriteFile(target, srcBytes, 0755)
		}
	}
}
