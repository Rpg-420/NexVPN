package core

import (
	"nexvpn/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsXhttpURI(t *testing.T) {
	cases := []struct {
		uri  string
		want bool
	}{
		{"vless://uuid@example.com:443?type=xhttp&security=tls&sni=example.com#x", true},
		{"vless://uuid@example.com:443?type=splithttp&security=tls#x", true},
		{"vless://uuid@example.com:443?type=ws&security=tls#x", false},
		{"vless://uuid@example.com:443?type=httpupgrade&path=%2F#x", false},
		{"vless://uuid@example.com:443#x", false},
		{"vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsInBvcnQiOjQ0MywiaWQiOiJ1dWlkIn0=", false},
		{"vless://38bf9c38-4b7b-4660-8686-c4249ad95c5b@zirsakht2.redjimbo.ir:5443?encryption=none&security=reality&type=xhttp&headerType=none&path=%2F&mode=stream-one&sni=aparat.com&fp=edge&pbk=Wa_qEGb5OZPF0T6PYZf7aMV0FjnWWC4Cpny_q80jx3Y&sid=de827e228483d333&spx=%2F#%F0%9F%87%AE%F0%9F%87%B7%20%F0%9F%87%A9%F0%9F%87%AA%20%F0%9D%99%B6%F0%9D%9A%8E%F0%9D%9A%9B%F0%9D%9A%96%F0%9D%9A%8A%F0%9D%9A%97%F0%9D%9A%A2%C2%B9%20%F", true},
	}
	for _, c := range cases {
		if got := isXhttpURI(c.uri); got != c.want {
			t.Errorf("isXhttpURI(%q) = %v, want %v", c.uri, got, c.want)
		}
	}
}

func TestDispatchXhttpDecide(t *testing.T) {
	d := NewSmartCoreDispatcher()
	uri := "vless://38bf9c38-4b7b-4660-8686-c4249ad95c5b@zirsakht2.redjimbo.ir:5443?encryption=none&security=reality&type=xhttp&headerType=none&path=%2F&mode=stream-one&sni=aparat.com&fp=edge&pbk=Wa_qEGb5OZPF0T6PYZf7aMV0FjnWWC4Cpny_q80jx3Y&sid=de827e228483d333&spx=%2F#%F0%9F%87%AE%F0%9F%87%B7%20%F0%9F%87%A9%F0%9F%87%AA%20%F0%9D%99%B6%F0%9D%9A%8E%F0%9D%9A%9B%F0%9D%9A%96%F0%9D%9A%8A%F0%9D%9A%97%F0%9D%9A%A2%C2%B9%20%F"
	profile := &config.Profile{
		ID:       "test-xhttp",
		Name:     "Test XHTTP Reality",
		Protocol: config.ProtocolXray,
		RawURI:   uri,
		Server:   "zirsakht2.redjimbo.ir",
		Port:     5443,
	}

	settings := config.Settings{
		ProxyMode:   "tun", // Even in TUN mode, xhttp must force Xray
		RoutingMode: "bypass_iran",
		V2RayCore:   "auto",
	}

	decision, err := d.Decide(profile, settings)
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	if decision.CoreProtocol != config.ProtocolXray {
		t.Fatalf("Expected core %v, got %v", config.ProtocolXray, decision.CoreProtocol)
	}

	t.Logf("Xray config generated:\n%s", decision.ConfigContent)

	xrayBin := filepath.Join("..", "..", "bin", "xray.exe")
	if _, statErr := os.Stat(xrayBin); statErr == nil {
		tmpFile, err := os.CreateTemp("", "xray-test-*.json")
		if err == nil {
			defer os.Remove(tmpFile.Name())
			_, _ = tmpFile.WriteString(decision.ConfigContent)
			_ = tmpFile.Close()

			absXray, _ := filepath.Abs(xrayBin)
			binDir, _ := filepath.Abs(filepath.Dir(xrayBin))
			cmd := exec.Command(absXray, "-test", "-config", tmpFile.Name())
			cmd.Dir = binDir
			cmd.Env = append(os.Environ(),
				"xray.location.asset="+binDir,
				"XRAY_LOCATION_ASSET="+binDir,
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("xray.exe -test failed: %v\nOutput: %s", err, string(output))
			}
			t.Logf("xray.exe validation passed:\n%s", string(output))
		}
	}
}
