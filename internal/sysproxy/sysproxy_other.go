//go:build !windows && !linux && !darwin

package sysproxy

import (
	"fmt"
	"os"
)

func setSystemProxyPlatform(host string, port int) error {
	return setFullSystemProxyPlatform(port, port)
}

func setFullSystemProxyPlatform(httpPort int, socksPort int) error {
	httpURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	_ = os.Setenv("HTTP_PROXY", httpURL)
	_ = os.Setenv("HTTPS_PROXY", httpURL)
	_ = os.Setenv("ALL_PROXY", socksURL)
	return nil
}

func clearSystemProxyPlatform() error {
	_ = os.Unsetenv("HTTP_PROXY")
	_ = os.Unsetenv("HTTPS_PROXY")
	_ = os.Unsetenv("ALL_PROXY")
	return nil
}
