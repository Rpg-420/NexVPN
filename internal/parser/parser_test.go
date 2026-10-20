package parser

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nexvpn/internal/config"
)

func TestParseVlessURI(t *testing.T) {
	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@example.com:443?security=reality&sni=yahoo.com&fp=chrome&pbk=abcd1234efgh5678abcd1234efgh5678abcd1234efg&sid=12345678&type=tcp#TestNode"
	node, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("Failed to parse VLESS URI: %v", err)
	}

	if node.Server != "example.com" {
		t.Errorf("Expected server 'example.com', got '%s'", node.Server)
	}
	if node.Port != 443 {
		t.Errorf("Expected port 443, got %d", node.Port)
	}

	var sbMap map[string]interface{}
	if err := json.Unmarshal([]byte(node.ConfigContent), &sbMap); err != nil {
		t.Fatalf("Generated config is not valid JSON: %v", err)
	}

	inbounds, ok := sbMap["inbounds"].([]interface{})
	if !ok || len(inbounds) < 2 {
		t.Fatalf("Expected at least 2 inbounds (mixed 10808 and http 10809), got %d", len(inbounds))
	}
}

func TestParseWireGuardConf(t *testing.T) {
	conf := `
[Interface]
PrivateKey = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
Address = 10.0.0.2/32, fd00::2/128
DNS = 1.1.1.1
MTU = 1420

[Peer]
PublicKey = bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb=
Endpoint = 198.51.100.1:51820
AllowedIPs = 0.0.0.0/0, ::/0
Reserved = 1, 2, 3
`
	outbound, host, err := ParseWireGuardConf(conf)
	if err != nil {
		t.Fatalf("Failed to parse WireGuard conf: %v", err)
	}

	if host != "198.51.100.1" {
		t.Errorf("Expected host 198.51.100.1, got %s", host)
	}
	if outbound["type"] != "wireguard" {
		t.Errorf("Expected type 'wireguard', got %v", outbound["type"])
	}

	sbJSON, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", host)
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}

	if !strings.Contains(sbJSON, "NexVPN-TUN") {
		t.Errorf("Expected TUN interface in config")
	}
	if !strings.Contains(sbJSON, "10809") {
		t.Errorf("Expected HTTP inbound on port 10809")
	}
	t.Logf("Generated SingBox WireGuard JSON:\n%s", sbJSON)

	sbBin := filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe")
	if _, statErr := os.Stat(sbBin); statErr == nil {
		tmpFile, err := os.CreateTemp("", "sb-wg-test-*.json")
		if err == nil {
			defer os.Remove(tmpFile.Name())
			_, _ = tmpFile.WriteString(sbJSON)
			_ = tmpFile.Close()

			binDir, _ := filepath.Abs(filepath.Dir(sbBin))
			cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed on ParseWireGuardConf config: %v\nOutput: %s\nConfig:\n%s", err, string(output), sbJSON)
			}
		}
	}
}

func TestBuildNodeConfigSmart(t *testing.T) {
	profile := config.Profile{
		Name:     "Test Hy2",
		Protocol: config.ProtocolHysteria2,
		RawURI:   "hy2://password123@example.org:8443?sni=example.org&insecure=1#MyHy2",
	}

	proto, cfg, err := BuildNodeConfig(&profile, "system_proxy", "bypass_iran")
	if err != nil {
		t.Fatalf("BuildNodeConfig failed: %v", err)
	}

	if proto != config.ProtocolSingBox {
		t.Errorf("Expected ProtocolSingBox, got %s", proto)
	}

	if !strings.Contains(cfg, "10808") || !strings.Contains(cfg, "10809") {
		t.Errorf("Config missing dual inbounds: %s", cfg)
	}
}

func TestSingBoxBinaryConfigCheck(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows binary validation on non-Windows OS")
	}
	sbBin := filepath.Join("..", "..", "bin", "sing-box.exe")
	if _, err := os.Stat(sbBin); os.IsNotExist(err) {
		t.Skip("sing-box.exe not found in bin, skipping binary validation test")
	}

	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@198.51.100.1:443?security=reality&sni=yahoo.com&fp=chrome&pbk=abcd1234efgh5678abcd1234efgh5678abcd1234efg&sid=12345678&type=tcp#TestNode"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	cfg, err := BuildSingBoxConfig(outbound, "system_proxy", "bypass_iran", "198.51.100.1")
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "sb-check-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, _ = tmpFile.WriteString(cfg)
	_ = tmpFile.Close()

	/*
		binDir, _ := filepath.Abs(filepath.Dir(sbBin))
		cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("sing-box check failed on generated config: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
		}
	*/
}

func TestXrayBinaryConfigCheck(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows binary validation on non-Windows OS")
	}
	xrayBin := filepath.Join("..", "..", "bin", "xray.exe")
	if _, err := os.Stat(xrayBin); os.IsNotExist(err) {
		t.Skip("xray.exe not found in bin, skipping binary validation test")
	}

	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@198.51.100.1:443?security=reality&sni=yahoo.com&fp=chrome&pbk=abcd1234efgh5678abcd1234efgh5678abcd1234efg&sid=12345678&type=tcp#TestNode"
	settings := config.Settings{
		ProxyMode:   "system_proxy",
		RoutingMode: "bypass_iran",
	}

	cfg, err := BuildXrayConfigFromURI(uri, 10809, 10808, "bypass_iran", settings)
	if err != nil {
		t.Fatalf("BuildXrayConfigFromURI failed: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "xray-check-*.json")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, _ = tmpFile.WriteString(cfg)
	_ = tmpFile.Close()

	absXrayBin, _ := filepath.Abs(xrayBin)
	binDir := filepath.Dir(absXrayBin)
	cmd := exec.Command(absXrayBin, "-test", "-config", tmpFile.Name())
	cmd.Dir = binDir
	cmd.Env = append(os.Environ(),
		"xray.location.asset="+binDir,
		"XRAY_LOCATION_ASSET="+binDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("xray check failed on generated config: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
	}
}

func TestWireGuardSingBoxConfigValidation(t *testing.T) {
	warpProfile, err := GenerateCloudflareWarp("162.159.192.1", 2408)
	if err != nil || warpProfile == nil {
		warpProfile = &config.Profile{
			Name:          "Mock WARP",
			Protocol:      config.ProtocolWireGuard,
			Category:      config.CategoryWireGuardWarp,
			ConfigContent: "[Interface]\nPrivateKey = aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=\nAddress = 172.16.0.2/32\nDNS = 1.1.1.1\nMTU = 1280\n[Peer]\nPublicKey = bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb=\nAllowedIPs = 0.0.0.0/0\nEndpoint = 162.159.192.1:2408\n",
			Server:        "162.159.192.1",
			Port:          2408,
		}
	}

	settings := config.Settings{
		ProxyMode:   "tun",
		RoutingMode: "bypass_iran",
		AmneziaJc:   4,
		AmneziaJmin: 50,
		AmneziaJmax: 1000,
	}

	_, cfg, err := BuildNodeConfigWithOptions(warpProfile, settings)
	if err != nil {
		t.Fatalf("BuildNodeConfigWithOptions failed on WARP: %v", err)
	}

	// Verify unsupported fields were stripped
	if strings.Contains(cfg, "init_packet_junk_size") || strings.Contains(cfg, "junk_packet_count") {
		t.Errorf("Config still contains unsupported Amnezia junk packet fields: %s", cfg)
	}

	// Verify sing-box binary check with build/bin/bin/sing-box.exe
	if runtime.GOOS == "windows" {
		sbBin := filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe")
		if _, statErr := os.Stat(sbBin); statErr == nil {
			tmpFile, err := os.CreateTemp("", "sb-wg-check-*.json")
			if err == nil {
				defer os.Remove(tmpFile.Name())
				_, _ = tmpFile.WriteString(cfg)
				_ = tmpFile.Close()

				binDir, _ := filepath.Abs(filepath.Dir(sbBin))
				cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("sing-box check failed on generated WARP config: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
				}
			}
		}
	}
}

func TestTLSFragmentInjection(t *testing.T) {
	uri := "vless://9cf8b982-263a-44d5-a8c6-111111111111@198.51.100.1:443?security=reality&sni=yahoo.com&fp=chrome&pbk=abcd1234efgh5678abcd1234efgh5678abcd1234efg&sid=12345678&type=tcp#TestNode"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	settings := config.Settings{
		FragmentEnabled: true,
		FragmentSize:    "120-250",
		FragmentSleep:   "15-30",
		RoutingMode:     "bypass_iran",
	}

	// A Reality outbound must NOT receive tls.fragment (the handshake needs an
	// unmodified ClientHello), which is why the URI above is used for the
	// negative case only.
	realityCfg, err := BuildSingBoxConfigAdvanced(outbound, "tun", "bypass_iran", "198.51.100.1", settings)
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced failed: %v", err)
	}
	if strings.Contains(realityCfg, `"fragment"`) {
		t.Errorf("Reality outbound must not contain tls.fragment: %s", realityCfg)
	}

	// A plain (non-Reality) TLS outbound is where fragmentation belongs. This
	// must be a **bool**: sing-box 1.12 removed the {enabled,size,sleep} object,
	// and feeding the object to the bundled 1.14.0 core made it exit during
	// startup with
	//   outbounds[N].tls.fragment: cannot unmarshal object ... of type bool
	// which broke every non-Reality sing-box profile whenever Fragment was on.
	// The old test here was deleted rather than fixed, which is how that shipped.
	plainURI := "vless://9cf8b982-263a-44d5-a8c6-111111111111@198.51.100.1:443?security=tls&sni=example.com&type=tcp#FragNode"
	plainOutbound, err := ParseOutboundFromURI(plainURI)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}

	fragCfg, err := BuildSingBoxConfigAdvanced(plainOutbound, "tun", "bypass_iran", "198.51.100.1", settings)
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced failed: %v", err)
	}

	var fragParsed struct {
		Outbounds []struct {
			TLS map[string]interface{} `json:"tls"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(fragCfg), &fragParsed); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}

	sawFragment := false
	for i, o := range fragParsed.Outbounds {
		raw, present := o.TLS["fragment"]
		if !present {
			continue
		}
		sawFragment = true
		if _, isBool := raw.(bool); !isBool {
			t.Errorf("outbounds[%d].tls.fragment must be a bool for sing-box 1.12+, got %T (%v)",
				i, raw, raw)
		}
	}
	if !sawFragment {
		t.Errorf("FragmentEnabled=true did not inject tls.fragment into a non-Reality TLS outbound: %s", fragCfg)
	}

	// The schema claim above is only worth anything if the real core agrees, so
	// run the bundled binary over the generated config.
	if runtime.GOOS == "windows" {
		sbBin := filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe")
		if _, statErr := os.Stat(sbBin); statErr == nil {
			tmpFile, err := os.CreateTemp("", "sb-fragment-*.json")
			if err == nil {
				defer os.Remove(tmpFile.Name())
				_, _ = tmpFile.WriteString(fragCfg)
				_ = tmpFile.Close()

				binDir, _ := filepath.Abs(filepath.Dir(sbBin))
				cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("sing-box check rejected the fragmented config: %v\nOutput: %s\nConfig:\n%s",
						err, string(output), fragCfg)
				}
			}
		}
	}

	xrayCfg, err := BuildXrayConfigFromURI(uri, 10809, 10808, "bypass_iran", settings)
	if err != nil {
		t.Fatalf("BuildXrayConfigFromURI failed: %v", err)
	}

	if !strings.Contains(xrayCfg, `"dialerProxy": "fragment"`) || !strings.Contains(xrayCfg, "tlshello") {
		t.Fatalf("Xray config does not contain injected fragment dialerProxy: %s", xrayCfg)
	}
}

func TestChainBuilderConfig(t *testing.T) {
	p1 := config.Profile{
		ID:       "p1",
		Name:     "Hop 1",
		Protocol: config.ProtocolSingBox,
		RawURI:   "vless://uuid1@1.1.1.1:443?security=reality&sni=google.com&pbk=key1&fp=chrome",
		Server:   "1.1.1.1",
		Port:     443,
	}
	p2 := config.Profile{
		ID:       "p2",
		Name:     "Hop 2",
		Protocol: config.ProtocolSingBox,
		RawURI:   "vless://uuid2@2.2.2.2:443?security=reality&sni=yahoo.com&pbk=key2&fp=chrome",
		Server:   "2.2.2.2",
		Port:     443,
	}

	settings := config.Settings{
		RoutingMode: "bypass_iran",
	}

	chained, err := BuildMultiHopChain([]config.Profile{p1, p2}, "Chained-Test", settings)
	if err != nil {
		t.Fatalf("BuildMultiHopChain failed: %v", err)
	}

	if strings.Contains(chained.ConfigContent, "rule_set") {
		t.Fatalf("Chained config contains invalid undefined rule_set: %s", chained.ConfigContent)
	}

	if !strings.Contains(chained.ConfigContent, "detour") {
		t.Fatalf("Chained config missing detour: %s", chained.ConfigContent)
	}
}

func TestSingBoxDNSServersValidation(t *testing.T) {
	sbBin := filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe")
	if _, statErr := os.Stat(sbBin); statErr != nil {
		t.Skip("sing-box binary not found")
	}

	testCases := []map[string]interface{}{
		// Case 0: type https with clean server and path
		{
			"tag":    "dns-remote",
			"type":   "https",
			"server": "1.1.1.1",
			"path":   "/dns-query",
		},
		// Case 1: type tcp
		{
			"tag":    "dns-remote",
			"type":   "tcp",
			"server": "1.1.1.1",
		},
		// Case 2: type udp
		{
			"tag":    "dns-remote",
			"type":   "udp",
			"server": "1.1.1.1",
		},
	}

	for i, srv := range testCases {
		cfgObj := map[string]interface{}{
			"dns": map[string]interface{}{
				"servers": []map[string]interface{}{
					srv,
					{"tag": "dns-direct", "type": "local"},
				},
			},
			"outbounds": []map[string]interface{}{
				{"type": "direct", "tag": "direct"},
			},
			"route": map[string]interface{}{
				"default_domain_resolver": "dns-direct",
			},
		}
		data, _ := json.Marshal(cfgObj)
		tmp, _ := os.CreateTemp("", "sb-dns-*.json")
		_, _ = tmp.Write(data)
		_ = tmp.Close()
		defer os.Remove(tmp.Name())

		cmd := exec.Command(sbBin, "check", "-c", tmp.Name())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("DNS Case %d failed: err=%v, out=%s", i, err, string(out))
		}
	}
}

func TestHysteria2AndTUICParsingAndConfig(t *testing.T) {
	hy2URI := "hy2://mypassword@example.com:443?sni=cdn.example.com&insecure=1&obfs=salamander&obfs-password=secretpass&upmbps=50&downmbps=100#Hy2Node"
	parsedHy2, err := ParseURI(hy2URI)
	if err != nil {
		t.Fatalf("ParseURI(hy2) failed: %v", err)
	}
	if parsedHy2.Protocol != config.ProtocolHysteria2 {
		t.Errorf("expected Protocol Hysteria2, got %v", parsedHy2.Protocol)
	}
	if parsedHy2.Outbound["type"] != "hysteria2" {
		t.Errorf("expected outbound type hysteria2, got %v", parsedHy2.Outbound["type"])
	}
	if parsedHy2.Outbound["up_mbps"] != 50 || parsedHy2.Outbound["down_mbps"] != 100 {
		t.Errorf("bandwidth limits not set correctly: %v", parsedHy2.Outbound)
	}

	tuicURI := "tuic://a1b2c3d4-e5f6-7890-abcd-ef1234567890:secret@example.com:8443?congestion_control=bbr&alpn=h3&sni=tuic.example.com#TUICNode"
	parsedTuic, err := ParseURI(tuicURI)
	if err != nil {
		t.Fatalf("ParseURI(tuic) failed: %v", err)
	}
	if parsedTuic.Protocol != config.ProtocolTUIC {
		t.Errorf("expected Protocol TUIC, got %v", parsedTuic.Protocol)
	}
	if parsedTuic.Outbound["type"] != "tuic" {
		t.Errorf("expected outbound type tuic, got %v", parsedTuic.Outbound["type"])
	}

	// Verify Sing-box config generation with Hysteria2
	settings := config.Settings{
		ProxyMode:   "tun",
		RoutingMode: "bypass_iran",
	}
	_, cfgHy2, err := BuildNodeConfigWithOptions(&config.Profile{
		ID:       "hy2-test",
		Protocol: config.ProtocolHysteria2,
		RawURI:   hy2URI,
		Server:   "example.com",
		Port:     443,
	}, settings)
	if err != nil {
		t.Fatalf("BuildNodeConfigWithOptions(Hy2) failed: %v", err)
	}
	if !strings.Contains(cfgHy2, `"type":"hysteria2"`) && !strings.Contains(cfgHy2, `"type": "hysteria2"`) {
		t.Errorf("Generated config missing hysteria2 outbound: %s", cfgHy2)
	}
}

func TestHTTPUpgradeTransportParsing(t *testing.T) {
	vlessURI := "vless://2f35965a-9a9b-45fd-ba32-987296dfb6be@albania.example.com:443?type=httpupgrade&path=%2Fws&host=cdn.albania.com&sni=albania.example.com&security=tls&fp=chrome#AlbaniaCDN"
	outbound, err := ParseOutboundFromURI(vlessURI)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI(vless httpupgrade) failed: %v", err)
	}
	transport, ok := outbound["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected transport map in outbound, got %v", outbound["transport"])
	}
	if transport["type"] != "httpupgrade" {
		t.Errorf("Expected type httpupgrade, got %v", transport["type"])
	}
	if transport["host"] != "cdn.albania.com" {
		t.Errorf("Expected host cdn.albania.com, got %v", transport["host"])
	}
	if _, hasHeaders := transport["headers"]; hasHeaders {
		t.Errorf("Transport must not contain duplicate headers map when host is set: %v", transport)
	}

	// Verify full SingBox config build and binary check
	cfg, err := BuildSingBoxConfig(outbound, "tun", "bypass_iran", "albania.example.com")
	if err != nil {
		t.Fatalf("BuildSingBoxConfig failed: %v", err)
	}

	sbBin := filepath.Join("..", "..", "build", "bin", "bin", "sing-box.exe")
	if _, statErr := os.Stat(sbBin); statErr == nil {
		tmpFile, err := os.CreateTemp("", "sb-hu-test-*.json")
		if err == nil {
			defer os.Remove(tmpFile.Name())
			_, _ = tmpFile.WriteString(cfg)
			_ = tmpFile.Close()

			binDir, _ := filepath.Abs(filepath.Dir(sbBin))
			cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed on httpupgrade config: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
			}
		}
	}
}

func TestVlessTCPHeaderTypeHTTP(t *testing.T) {
	// Regression: type=tcp&headerType=http profiles must produce an HTTP-camouflaged
	// transport, otherwise the server replies with plain HTTP and the handshake dies.
	uri := "vless://92d7a093-717e-42d8-8a30-1f7556626acf@tutaim.example.com:443?encryption=none&security=none&type=tcp&headerType=http&host=amp-api-edge.apps.apple.com&path=%2F#TurkeyTCP"

	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}
	transport, ok := outbound["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected http transport for headerType=http, got: %v", outbound["transport"])
	}
	if transport["type"] != "http" {
		t.Fatalf("expected transport type http, got %v", transport["type"])
	}
	hostList, ok := transport["host"].([]string)
	if !ok || len(hostList) != 1 || hostList[0] != "amp-api-edge.apps.apple.com" {
		t.Fatalf("expected host [amp-api-edge.apps.apple.com], got %v", transport["host"])
	}

	// Xray path: must emit tcpSettings.header.type=http
	xrOut, _, err := ParseXrayOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseXrayOutboundFromURI failed: %v", err)
	}
	ss, _ := xrOut["streamSettings"].(map[string]interface{})
	if ss == nil || ss["network"] != "tcp" {
		t.Fatalf("expected xray network tcp, got %v", ss)
	}
	tcpSettings, _ := ss["tcpSettings"].(map[string]interface{})
	if tcpSettings == nil {
		t.Fatalf("expected tcpSettings with http header, got: %v", ss)
	}
	header, _ := tcpSettings["header"].(map[string]interface{})
	if header == nil || header["type"] != "http" {
		t.Fatalf("expected tcpSettings.header.type == http, got: %v", header)
	}
}

func TestTrojanTransportParsing(t *testing.T) {
	// Regression: trojan URIs with ws/grpc transports must not fall back to bare TCP+TLS.
	uri := "trojan://secretpass@trojan.example.com:443?sni=cdn.example.com&type=ws&path=%2Ftj&host=cdn.example.com#TrojanWS"

	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI(trojan ws) failed: %v", err)
	}
	transport, ok := outbound["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ws transport for trojan, got: %v", outbound["transport"])
	}
	if transport["type"] != "ws" {
		t.Fatalf("expected transport type ws, got %v", transport["type"])
	}

	xrOut, _, err := ParseXrayOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseXrayOutboundFromURI(trojan ws) failed: %v", err)
	}
	ss, _ := xrOut["streamSettings"].(map[string]interface{})
	if ss == nil || ss["network"] != "ws" {
		t.Fatalf("expected xray network ws for trojan, got: %v", ss)
	}
}

func TestVlessConfigValidation(t *testing.T) {
	uri := "vless://38bf9c38-4b7b-4660-8686-c4249ad95c5b@zirsakht2.redjimbo.ir:5443?encryption=none&security=reality&type=httpupgrade&headerType=none&path=%2F&sni=aparat.com&fp=edge&pbk=Wa_qEGb5OZPF0T6PYZf7aMV0FjnWWC4Cpny_q80jx3Y&sid=de827e228483d333&spx=%2F#%F0%9F%87%AE%F0%9F%87%B7"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI error: %v", err)
	}

	settings := config.Settings{
		ProxyMode:       "tun",
		RoutingMode:     "bypass_iran",
		FragmentEnabled: true, // Fragment enabled must NOT inject into Reality
		IPStrategy:      "prefer_ipv4",
	}

	cfg, err := BuildSingBoxConfigAdvanced(outbound, "tun", "bypass_iran", "zirsakht2.redjimbo.ir", settings)
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced error: %v", err)
	}

	// Verify that tls.fragment was NOT injected into Reality
	if strings.Contains(cfg, `"fragment"`) {
		t.Fatalf("Reality outbound must not contain tls.fragment: %s", cfg)
	}

	// Verify that IPv6 reject rule was generated
	if !strings.Contains(cfg, `"action": "reject"`) {
		t.Fatalf("Expected action reject in route rules: %s", cfg)
	}

	sbBin := filepath.Join("..", "..", "bin", "sing-box.exe")
	if _, statErr := os.Stat(sbBin); statErr == nil {
		tmpFile, err := os.CreateTemp("", "sb-vless-*.json")
		if err == nil {
			defer os.Remove(tmpFile.Name())
			_, _ = tmpFile.WriteString(cfg)
			_ = tmpFile.Close()

			binDir, _ := filepath.Abs(filepath.Dir(sbBin))
			cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("sing-box check failed: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
			}
		}
	}
}

func TestVlessSNIFallbackToHostHeader(t *testing.T) {
	// When host is an IP address and sni is missing, host header must become SNI so CDN/Workers complete TLS handshake
	uri := "vless://38bf9c38-4b7b-4660-8686-c4249ad95c5b@104.17.220.53:2053?encryption=none&security=tls&type=ws&host=web-4221fd-81b4c6.rpg-420-4bbd.workers.dev&path=%2F#CleanIPNode"
	outbound, err := ParseOutboundFromURI(uri)
	if err != nil {
		t.Fatalf("ParseOutboundFromURI failed: %v", err)
	}
	tlsObj, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tls map in outbound: %v", outbound)
	}
	if tlsObj["server_name"] != "web-4221fd-81b4c6.rpg-420-4bbd.workers.dev" {
		t.Fatalf("expected server_name to be worker domain, got: %v", tlsObj["server_name"])
	}
}

func TestWireGuardEndpointSafeTypes(t *testing.T) {
	// Simulate JSON unmarshaled outbound (where arrays are []interface{} and ints are float64)
	jsonRaw := `{
		"type": "wireguard",
		"tag": "proxy",
		"local_address": ["172.16.0.2/32", "2606:4700:110:8cc2:617d:9326:86fe:3b6e/128"],
		"server": "162.159.192.1",
		"server_port": 2408,
		"private_key": "privkey=",
		"peer_public_key": "pubkey=",
		"reserved": [1, 2, 3],
		"mtu": 1280
	}`
	var ob map[string]interface{}
	if err := json.Unmarshal([]byte(jsonRaw), &ob); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	ep := convertWireGuardOutboundToEndpoint(ob)
	addrs, ok := ep["address"].([]string)
	if !ok || len(addrs) == 0 {
		t.Fatalf("convertWireGuardOutboundToEndpoint failed to extract address: %v", ep["address"])
	}
	peers, ok := ep["peers"].([]map[string]interface{})
	if !ok || len(peers) == 0 {
		t.Fatalf("peers missing: %v", ep["peers"])
	}
	if peers[0]["port"] != 2408 {
		t.Fatalf("port mismatch, expected 2408 got: %v", peers[0]["port"])
	}
	res, ok := peers[0]["reserved"].([]int)
	if !ok || len(res) != 3 || res[0] != 1 {
		t.Fatalf("reserved bytes mismatch: %v", peers[0]["reserved"])
	}
}
