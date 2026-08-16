//go:build linux

package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
)

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

	// 1. GNOME Desktop GSettings
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "port", fmt.Sprintf("%d", httpPort)).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "port", fmt.Sprintf("%d", httpPort)).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "host", "127.0.0.1").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.socks", "port", fmt.Sprintf("%d", socksPort)).Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "ignore-hosts", "['localhost', '127.0.0.0/8', '::1', '*.ir', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16']").Run()

	// 2. KDE Plasma kwriteconfig5
	_ = exec.Command("kwriteconfig5", "--file", "kioslaurc", "--group", "Proxy Settings", "--key", "ProxyType", "1").Run()
	_ = exec.Command("kwriteconfig5", "--file", "kioslaurc", "--group", "Proxy Settings", "--key", "httpProxy", fmt.Sprintf("http://127.0.0.1:%d", httpPort)).Run()
	_ = exec.Command("kwriteconfig5", "--file", "kioslaurc", "--group", "Proxy Settings", "--key", "httpsProxy", fmt.Sprintf("http://127.0.0.1:%d", httpPort)).Run()
	_ = exec.Command("kwriteconfig5", "--file", "kioslaurc", "--group", "Proxy Settings", "--key", "socksProxy", fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)).Run()

	// 3. Linux Shell Environment Variables
	httpURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	_ = os.Setenv("HTTP_PROXY", httpURL)
	_ = os.Setenv("HTTPS_PROXY", httpURL)
	_ = os.Setenv("ALL_PROXY", socksURL)
	_ = os.Setenv("http_proxy", httpURL)
	_ = os.Setenv("https_proxy", httpURL)
	_ = os.Setenv("all_proxy", socksURL)
	_ = os.Setenv("NO_PROXY", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	_ = os.Setenv("no_proxy", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

	return nil
}

func clearSystemProxyPlatform() error {
	// GNOME
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()

	// KDE
	_ = exec.Command("kwriteconfig5", "--file", "kioslaurc", "--group", "Proxy Settings", "--key", "ProxyType", "0").Run()

	// Env
	_ = os.Unsetenv("HTTP_PROXY")
	_ = os.Unsetenv("HTTPS_PROXY")
	_ = os.Unsetenv("ALL_PROXY")
	_ = os.Unsetenv("http_proxy")
	_ = os.Unsetenv("https_proxy")
	_ = os.Unsetenv("all_proxy")
	_ = os.Unsetenv("NO_PROXY")
	_ = os.Unsetenv("no_proxy")

	return nil
}
