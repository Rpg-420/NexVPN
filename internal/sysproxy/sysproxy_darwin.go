//go:build darwin

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getDarwinActiveServices() []string {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return []string{"Wi-Fi", "Ethernet"}
	}
	lines := strings.Split(string(out), "\n")
	var services []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" && !strings.Contains(trimmed, "*") {
			services = append(services, trimmed)
		}
	}
	if len(services) == 0 {
		return []string{"Wi-Fi", "Ethernet"}
	}
	return services
}

// isStaleNexVPNProxy: on macOS, clearSystemProxyPlatform only flips each network
// service's proxy toggle off (it never overwrites a stored third-party proxy host),
// so unconditionally allowing the cleanup here carries much less risk than on Windows.
func isStaleNexVPNProxy() bool {
	return true
}

// IsSystemProxyEnabled reports whether any active network service has its web
// proxy switched on.
func IsSystemProxyEnabled() bool {
	for _, svc := range getDarwinActiveServices() {
		out, err := exec.Command("networksetup", "-getwebproxy", svc).Output()
		if err == nil && strings.Contains(strings.ToLower(string(out)), "enabled: yes") {
			return true
		}
	}
	return false
}

func setSystemProxyPlatform(host string, port int) error {
	if host == "" {
		host = "127.0.0.1"
	}
	return setFullSystemProxyPlatform(port, port)
}

func setFullSystemProxyPlatform(httpPort int, socksPort int) error {
	if httpPort <= 0 {
		httpPort = 10809
	}
	if socksPort <= 0 {
		socksPort = 10808
	}

	services := getDarwinActiveServices()
	for _, svc := range services {
		_ = exec.Command("networksetup", "-setwebproxy", svc, "127.0.0.1", fmt.Sprintf("%d", httpPort)).Run()
		_ = exec.Command("networksetup", "-setsecurewebproxy", svc, "127.0.0.1", fmt.Sprintf("%d", httpPort)).Run()
		_ = exec.Command("networksetup", "-setsocksfirewallproxy", svc, "127.0.0.1", fmt.Sprintf("%d", socksPort)).Run()
		_ = exec.Command("networksetup", "-setwebproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "on").Run()
		_ = exec.Command("networksetup", "-setproxybypassdomains", svc, "127.0.0.1", "localhost", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12", "*.ir").Run()
	}

	httpURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	_ = os.Setenv("HTTP_PROXY", httpURL)
	_ = os.Setenv("HTTPS_PROXY", httpURL)
	_ = os.Setenv("ALL_PROXY", socksURL)
	_ = os.Setenv("http_proxy", httpURL)
	_ = os.Setenv("https_proxy", httpURL)
	_ = os.Setenv("all_proxy", socksURL)

	return nil
}

func clearSystemProxyPlatform() error {
	services := getDarwinActiveServices()
	for _, svc := range services {
		_ = exec.Command("networksetup", "-setwebproxystate", svc, "off").Run()
		_ = exec.Command("networksetup", "-setsecurewebproxystate", svc, "off").Run()
		_ = exec.Command("networksetup", "-setsocksfirewallproxystate", svc, "off").Run()
	}

	_ = os.Unsetenv("HTTP_PROXY")
	_ = os.Unsetenv("HTTPS_PROXY")
	_ = os.Unsetenv("ALL_PROXY")
	_ = os.Unsetenv("http_proxy")
	_ = os.Unsetenv("https_proxy")
	_ = os.Unsetenv("all_proxy")

	return nil
}
