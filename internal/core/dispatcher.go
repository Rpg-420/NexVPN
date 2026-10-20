package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/parser"
	"nexvpn/internal/protocols"
	"nexvpn/internal/sysproc"
)

// isXhttpURI delegates to the parser package so the xhttp detection logic
// lives in exactly one place. Xray-only transports must be forced onto the
// Xray core to avoid "v2ray-http-upgrade: unexpected status: 400 Bad Request".
func isXhttpURI(raw string) bool {
	return parser.IsXhttpURI(raw)
}

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

	// Psiphon / Emergency Shield Profile
	if profile.Protocol == config.ProtocolPsiphon || profile.Category == config.CategoryPsiphon {
		// Dispatch to the native PsiphonAdapter whenever the core binary is
		// present.
		//
		// This used to be gated on protocols.HasPsiphonServers(), which asks
		// whether a *cached* server list already exists on disk. On a fresh
		// install nothing is cached yet, so the gate was always false and every
		// Psiphon connection silently became a Sing-box "Emergency Shield"
		// tunnel — the user asked for Psiphon and got sing-box, with nothing in
		// the UI to say so.
		//
		// The gate is also unnecessary: the adapter hands the core a
		// RemoteServerListURL, and the core bootstraps its own server list on
		// first run (verified: it downloads and imports the list, then exposes
		// SOCKS 10808 / HTTP 10809). A cached list only makes later runs faster.
		if psiAdapter, err := protocols.NewAdapter(config.ProtocolPsiphon); err == nil {
			if _, binErr := protocols.FindBinary("psiphon-tunnel-core"); binErr == nil {
				return &DispatchDecision{
					CoreProtocol:  config.ProtocolPsiphon,
					ConfigContent: profile.ConfigContent,
					Adapter:       psiAdapter,
					CoreName:      "Psiphon Core",
					Description:   "Native Psiphon circumvention tunnel (SSH/OSSH/QUIC)",
				}, nil
			} else if _, binErr2 := protocols.FindBinary("psiphon"); binErr2 == nil {
				return &DispatchDecision{
					CoreProtocol:  config.ProtocolPsiphon,
					ConfigContent: profile.ConfigContent,
					Adapter:       psiAdapter,
					CoreName:      "Psiphon Core",
					Description:   "Native Psiphon circumvention tunnel (SSH/OSSH/QUIC)",
				}, nil
			}
		}

		// The Psiphon core binary is missing entirely. Only then fall back to
		// the in-engine Sing-box emergency tunnel, and say so plainly instead of
		// presenting it as Psiphon.
		diary.Log("connect", "psiphon-core-missing", "fallback", "emergency-shield")
		return DispatchEmergencyShield(settings, proxyMode, routingMode)
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

		// XHTTP/SplitHTTP is an Xray-only transport: the bundled sing-box rejects it
		// with "unknown transport type: xhttp", and approximating it with httpupgrade
		// makes every connection answer 400 Bad Request. Force the Xray core for
		// these profiles regardless of the user's core/TUN preference.
		forceXray := isXhttpURI(uri)

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
		if (proxyMode == "tun" || isSingBoxPreferred || v2rayCorePref == "singbox") && !forceXray {
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
				ConfigContent: sanitizeRawSingBoxJSON(profile.ConfigContent),
				Adapter:       adapter,
				CoreName:      "Sing-box Core (Custom JSON)",
				Description:   "Custom Sing-box JSON configuration",
			}, nil
		}
	}

	return nil, fmt.Errorf("unable to determine core or generate valid configuration for profile: %s", profile.Name)
}

// sanitizeRawSingBoxJSON cleans up stale/malformed legacy Sing-box JSON configs
func sanitizeRawSingBoxJSON(rawJSON string) string {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &cfg); err != nil {
		return rawJSON
	}

	// 1. Sanitize DNS servers
	if dns, ok := cfg["dns"].(map[string]interface{}); ok {
		if servers, ok := dns["servers"].([]interface{}); ok {
			for _, s := range servers {
				if sMap, ok := s.(map[string]interface{}); ok {
					if sType, _ := sMap["type"].(string); sType == "local" {
						delete(sMap, "detour")
						sMap["type"] = "udp"
						sMap["server"] = "8.8.8.8"
					}
				}
			}
		}
	}

	// 2. Sanitize WireGuard endpoints
	if endpoints, ok := cfg["endpoints"].([]interface{}); ok {
		for _, ep := range endpoints {
			if epMap, ok := ep.(map[string]interface{}); ok {
				if epType, _ := epMap["type"].(string); epType == "wireguard" {
					delete(epMap, "detour")
					if addrs, ok := epMap["address"].([]interface{}); ok {
						var v4Addrs []interface{}
						for _, a := range addrs {
							if aStr, ok := a.(string); ok && !strings.Contains(aStr, ":") {
								v4Addrs = append(v4Addrs, aStr)
							}
						}
						if len(v4Addrs) > 0 {
							epMap["address"] = v4Addrs
						}
					}
					if peers, ok := epMap["peers"].([]interface{}); ok && len(peers) > 0 {
						if pMap, ok := peers[0].(map[string]interface{}); ok {
							pMap["allowed_ips"] = []string{"0.0.0.0/0"}
						}
					}
				}
			}
		}
	}

	// 3. Sanitize outbounds (httpupgrade duplicate Host header)
	if outbounds, ok := cfg["outbounds"].([]interface{}); ok {
		for _, ob := range outbounds {
			if obMap, ok := ob.(map[string]interface{}); ok {
				if tr, ok := obMap["transport"].(map[string]interface{}); ok {
					if trType, _ := tr["type"].(string); trType == "httpupgrade" || trType == "xhttp" || trType == "splithttp" {
						if _, hasHost := tr["host"]; hasHost {
							delete(tr, "headers")
						}
					}
				}
			}
		}
	}

	sanitized, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return rawJSON
	}
	return string(sanitized)
}

// wintunCandidates returns the locations where wintun.dll is looked up, in
// priority order. Kept in one place so the engine, the installer path and the
// diagnostics report can never drift apart.
func wintunCandidates() []string {
	exe, err := os.Executable()
	if err != nil {
		return []string{"bin/wintun.dll", "wintun.dll"}
	}
	execFolder := filepath.Dir(exe)
	return []string{
		filepath.Join(execFolder, "bin", "wintun.dll"),
		filepath.Join(execFolder, "wintun.dll"),
		"bin/wintun.dll",
		"wintun.dll",
	}
}

// ResolveWintunDLL returns the path and size of the first usable wintun.dll, or
// ("", 0) when the driver is genuinely unavailable.
func ResolveWintunDLL() (string, int64) {
	for _, c := range wintunCandidates() {
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 10000 {
			return c, info.Size()
		}
	}
	return "", 0
}

// EnsureWintunDriver checks and ensures wintun.dll is present in working directories
func EnsureWintunDriver() {
	validSource, _ := ResolveWintunDLL()
	if validSource == "" {
		return
	}

	execDir, err := os.Executable()
	if err != nil {
		return
	}
	execFolder := filepath.Dir(execDir)
	binDir := filepath.Join(execFolder, "bin")

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

// ReleaseStaleTUNAdapter re-enables only virtual adapters owned by NexVPN.
// Third-party Wintun adapters must never be reset by this application.
func ReleaseStaleTUNAdapter() {
	if runtime.GOOS != "windows" {
		return
	}
	adapterNames := []string{"NexVPN-TUN", "NexVPN-OVPN"}
	for _, name := range adapterNames {
		// Disable first to reset any stale state
		disableCmd := exec.Command("netsh", "interface", "set", "interface", name, "admin=disable")
		disableCmd.SysProcAttr = sysproc.HideWindowAttr()
		_ = disableCmd.Run()
		// Enable the adapter
		enableCmd := exec.Command("netsh", "interface", "set", "interface", name, "admin=enable")
		enableCmd.SysProcAttr = sysproc.HideWindowAttr()
		_ = enableCmd.Run()
	}
}

// tunModeRequested reports whether this connection will ask a core to create a
// TUN adapter. An empty ProxyMode means "auto", which resolves to TUN.
func tunModeRequested(settings config.Settings) bool {
	return settings.ProxyMode == "" || settings.ProxyMode == "tun"
}

// tunInterfaceCandidates are the adapter names a TUN core may be told to use, in
// preference order. The suffixes exist so a ghost on one name cannot block every
// future connection: the engine walks the list until it finds a name that is not
// ghosted.
func tunInterfaceCandidates() []string {
	names := make([]string, 0, 9)
	names = append(names, "NexVPN-TUN")
	for i := 2; i <= 9; i++ {
		names = append(names, fmt.Sprintf("NexVPN-TUN-%d", i))
	}
	return names
}

// chooseTunInterfaceName returns an adapter name that is safe to hand to a core.
//
// See PrepareTUNForConnect for why a name can be unsafe. The scan is done once
// for all candidates — each half of it costs a PowerShell launch, and there are
// up to ten names to check. Ghosting is skipped entirely when the machine's
// adapter state cannot be read: a probe that did not run must not be able to
// rename adapters.
func chooseTunInterfaceName() string {
	return chooseTunInterfaceNameFromScan(protocols.ScanWintunGhosts())
}

// chooseTunInterfaceNameFromScan is chooseTunInterfaceName against a scan the
// caller already paid for, so PrepareTUNForConnect does not probe the machine
// twice.
func chooseTunInterfaceNameFromScan(scan protocols.WintunGhostScan) string {
	candidates := tunInterfaceCandidates()

	for _, name := range candidates {
		if !scan.Ghosted(name) {
			if name != candidates[0] {
				diary.Log("connect", "tun-ghost-avoided", "ghost", candidates[0], "using", name)
			}
			return name
		}
	}

	// Every candidate is ghosted, which means the state could not be read at all
	// or the machine is in a state this check cannot describe. Returning the
	// default keeps the previous behaviour rather than inventing a name the rest
	// of the app does not know.
	diary.Log("connect", "tun-ghost-avoided", "ghost", candidates[0], "using", candidates[0], "note", "every candidate is ghosted")
	return candidates[0]
}

// PrepareTUNForConnect makes sure a TUN core starts against a clean adapter, and
// returns the settings a core should be built with.
//
// Two separate defects are handled here, both of which made a dying core look
// connected and sent the engine into a pointless server-switching loop.
//
// 1. A *live* leftover adapter. The readiness probe asks only "does an adapter
// with this name exist?", and a leftover one answers yes instantly. The diary
// shows what that cost: a `tun-ready` line 0.3-2s after spawn, against the ~10s
// a real Wintun adapter takes to come up, followed by `core-exited: exit status
// 1`. Deleting the adapter first removes the ambiguity.
//
// 2. A *ghost* adapter. Its name is still bound in the network class registry to
// a Wintun software device, but Windows cannot enumerate an adapter under it, so
// `Get-NetAdapter` and `Remove-NetAdapter` never see it and the Wintun DLL does
// not answer for it either. `openvpn --show-adapters` still lists it, which is
// how it was found. The remedy is to use a different name, which is why the
// returned settings carry a TunInterfaceName; RemoveStaleTUNAdapter also removes
// the binding directly, now that the app runs elevated.
//
// Only NexVPN's own adapters are touched (see RemoveStaleTUNAdapter).
func PrepareTUNForConnect(settings config.Settings) config.Settings {
	if runtime.GOOS != "windows" {
		ReleaseStaleTUNAdapter()
		return settings
	}
	if !tunModeRequested(settings) {
		// A proxy-only core never creates an adapter, so there is nothing to
		// clean and no reason to pay for the PowerShell round trip.
		return settings
	}

	scan := protocols.ScanWintunGhosts()
	settings.TunInterfaceName = chooseTunInterfaceNameFromScan(scan)
	diary.Log("connect", "tun-prepare", "mode", settings.ProxyMode, "adapter", settings.TunInterfaceName)

	// Only pay for the removal when there is actually something of ours to
	// remove. RemoveStaleTUNAdapter costs a PowerShell launch plus a 900 ms
	// settle, and it used to run on every connect even on a machine with no
	// leftover adapter at all — which, together with the ghost scan, is where
	// the ~15 s `tun-prepare` phase in the diary came from.
	if staleAdaptersPresent(scan) {
		RemoveStaleTUNAdapter()
		// Windows needs a beat to retire the PnP device before the core
		// re-creates it; without this the new adapter can inherit the old node's
		// state, which is the very thing being cleaned up.
		time.Sleep(900 * time.Millisecond)
	} else {
		diary.Log("connect", "tun-removal-skipped")
	}

	return settings
}

// staleAdaptersPresent reports whether a scan saw any adapter name belonging to
// NexVPN, either as a live adapter or as a registry binding.
//
// A binding alone is enough: that is exactly the ghost case, and it is the one
// removal exists for. Names belonging to other vendors cannot appear here
// because only `SWD\Wintun\` bindings are collected, and they are then matched
// against NexVPN's own candidate list.
func staleAdaptersPresent(scan protocols.WintunGhostScan) bool {
	// The name list lives in the protocols package so that the presence check and
	// the removal pass can never drift apart; see NexVPNWintunAdapterNames.
	return protocols.NexVPNWintunAdapterPresent(scan)
}

// RemoveStaleTUNAdapter deletes NexVPN's own Wintun adapters outright.
//
// ReleaseStaleTUNAdapter only toggles the admin state, which cannot repair the
// condition that actually blocks a reconnect: a *ghost* adapter, whose name is
// still bound in the network class registry to a Wintun software device while
// Windows has no adapter under it. The core then dies with
//
//	configure tun interface: (create adapter: Cannot create a file when that
//	file already exists. | open existing adapter: Element not found.)
//
// and no amount of netsh toggling helps, because the interface is not in the
// adapter list for netsh to find in the first place. So this does three passes:
//
//  1. `Remove-NetAdapter` for a live-but-unwanted adapter.
//  2. `Remove-PnpDevice` by the PnP instance id recorded in the registry binding
//     — the only handle a ghost has, and the reason a ghost is now removable at
//     all rather than merely avoidable.
//  3. `Remove-PnpDevice` by friendly name, for the case where the device is
//     present but not enumerable as a network adapter.
//
// Scoped strictly by exact name to NexVPN's own adapters — third-party Wintun
// adapters (WireGuard, other VPN clients) are never touched.
func RemoveStaleTUNAdapter() {
	protocols.ClearStaleNexVPNWintunAdapters()
}

// DispatchEmergencyShield constructs an in-engine Emergency Multi-Hop / Censorship Circumvention tunnel
// using Sing-Box and Cloudflare WARP with TLS Fragment and Direct fallback.
func DispatchEmergencyShield(settings config.Settings, proxyMode, routingMode string) (*DispatchDecision, error) {
	if runtime.GOOS == "windows" && !sysproc.IsAdmin() && (proxyMode == "tun" || proxyMode == "") {
		proxyMode = "system_proxy"
	}

	warpCleanIP := settings.WarpCleanIP
	if warpCleanIP == "162.159.192.1" {
		warpCleanIP = ""
	}
	warpPort := settings.WarpPort
	if warpPort == 2408 {
		warpPort = 0
	}

	warpProfile, err := parser.GenerateCloudflareWarp(warpCleanIP, warpPort)
	if err == nil && warpProfile != nil && warpProfile.RawURI != "" {
		outbound, pErr := parser.ParseOutboundFromURI(warpProfile.RawURI)
		if pErr == nil && outbound != nil {
			targetIP := warpCleanIP
			if targetIP == "" && warpProfile.Server != "" {
				targetIP = warpProfile.Server
			}
			sbJSON, sbErr := parser.BuildSingBoxConfigAdvanced(outbound, proxyMode, routingMode, targetIP, settings)
			if sbErr == nil {
				adapter, aErr := protocols.NewAdapter(config.ProtocolSingBox)
				if aErr == nil {
					return &DispatchDecision{
						CoreProtocol:  config.ProtocolSingBox,
						ConfigContent: sbJSON,
						Adapter:       adapter,
						CoreName:      "Sing-box (Emergency Shield Core)",
						Description:   "In-Engine Emergency Multi-Hop Censorship Circumvention Tunnel",
					}, nil
				}
			}
		}
	}

	// Fallback to in-engine TLS Fragment emergency direct circumvention
	emergencySettings := settings
	emergencySettings.FragmentEnabled = true
	emergencySettings.FragmentSize = "10-30"
	emergencySettings.FragmentSleep = "2-5"

	directOutbound := map[string]interface{}{
		"type": "direct",
		"tag":  "proxy",
	}
	sbJSON, sbErr := parser.BuildSingBoxConfigAdvanced(directOutbound, proxyMode, routingMode, "1.1.1.1", emergencySettings)
	if sbErr == nil {
		adapter, aErr := protocols.NewAdapter(config.ProtocolSingBox)
		if aErr == nil {
			return &DispatchDecision{
				CoreProtocol:  config.ProtocolSingBox,
				ConfigContent: sbJSON,
				Adapter:       adapter,
				CoreName:      "Sing-box (Emergency Shield Core)",
				Description:   "In-Engine Emergency TLS Fragment Censorship Circumvention Tunnel",
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to build emergency shield configuration")
}
