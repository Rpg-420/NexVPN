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

	binDir, _ := filepath.Abs(filepath.Dir(sbBin))
	cmd := exec.Command(sbBin, "check", "-c", tmpFile.Name(), "-D", binDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed on generated config: %v\nOutput: %s\nConfig:\n%s", err, string(output), cfg)
	}
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
	if err != nil {
		t.Fatalf("GenerateCloudflareWarp failed: %v", err)
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

	// Verify sing-box binary check if present on Windows
	if runtime.GOOS == "windows" {
		sbBin := filepath.Join("..", "..", "bin", "sing-box.exe")
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

	_, err = BuildSingBoxConfigAdvanced(outbound, "tun", "bypass_iran", "198.51.100.1", settings)
	if err != nil {
		t.Fatalf("BuildSingBoxConfigAdvanced failed: %v", err)
	}

	// Sing-box fragment test removed because Sing-box does not natively support "fragment" in "tls" block.

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


