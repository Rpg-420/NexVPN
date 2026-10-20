//go:build windows

package sysproxy

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var (
	modwininet            = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOption = modwininet.NewProc("InternetSetOptionW")
)

const (
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
const connectionsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings\Connections`
const defaultBypass = `localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*;*.ir;<local>`

func setSystemProxyPlatform(host string, port int) error {
	if host == "" {
		host = "127.0.0.1"
	}
	if host != "127.0.0.1" && host != "localhost" {
		proxyAddr := fmt.Sprintf("http=%s:%d;https=%s:%d;socks=%s:%d", host, port, host, port, host, port)
		k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("open internet settings registry key: %w", err)
		}
		defer func() { _ = k.Close() }()
		if err := writeProxySettings(k, 1, proxyAddr, defaultBypass); err != nil {
			return err
		}
		notifyWinInet()
		return nil
	}
	return setFullSystemProxyPlatform(port, 10808)
}

// writeProxySettings applies the three core Internet Settings values and
// returns the first failure instead of silently swallowing it. A failed proxy
// write used to leave the user believing traffic was tunnelled when it was not.
func writeProxySettings(k registry.Key, enable uint32, server string, override string) error {
	if err := k.SetDWordValue("ProxyEnable", enable); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", server); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", override); err != nil {
		return fmt.Errorf("set ProxyOverride: %w", err)
	}
	return nil
}

func setFullSystemProxyPlatform(httpPort int, socksPort int) error {
	if httpPort <= 0 {
		httpPort = 10809
	}
	if socksPort <= 0 {
		socksPort = 10808
	}

	proxyAddr := fmt.Sprintf("http=127.0.0.1:%d;https=127.0.0.1:%d;socks=127.0.0.1:%d", httpPort, httpPort, socksPort)

	// 1. Update Internet Settings Registry Key
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open internet settings registry key: %w", err)
	}
	if err := writeProxySettings(k, 1, proxyAddr, defaultBypass); err != nil {
		_ = k.Close()
		return err
	}
	// Removing AutoConfigURL is best-effort: the value often does not exist, and
	// that is not a failure. Any other error still surfaces via SetXxxValue above.
	_ = k.DeleteValue("AutoConfigURL")
	_ = k.Close()

	// 2. Update Connections binary blob (DefaultConnectionSettings & SavedLegacySettings)
	ck, err := registry.OpenKey(registry.CURRENT_USER, connectionsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open connections registry key: %w", err)
	}
	blob := buildConnectionBlob(true, proxyAddr, defaultBypass)
	if err := ck.SetBinaryValue("DefaultConnectionSettings", blob); err != nil {
		_ = ck.Close()
		return fmt.Errorf("set DefaultConnectionSettings: %w", err)
	}
	if err := ck.SetBinaryValue("SavedLegacySettings", blob); err != nil {
		_ = ck.Close()
		return fmt.Errorf("set SavedLegacySettings: %w", err)
	}
	_ = ck.Close()

	// 3. Set environment variables for shell tools and CLI processes.
	// Setenv only fails on an invalid name/value (a compile-time constant here),
	// so a failure is recorded but never aborts the proxy configuration.
	httpProxyURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksProxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	const noProxyList = "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	envPairs := [][2]string{
		{"HTTP_PROXY", httpProxyURL}, {"HTTPS_PROXY", httpProxyURL}, {"ALL_PROXY", socksProxyURL},
		{"http_proxy", httpProxyURL}, {"https_proxy", httpProxyURL}, {"all_proxy", socksProxyURL},
		{"NO_PROXY", noProxyList}, {"no_proxy", noProxyList},
	}
	var envErr error
	for _, kv := range envPairs {
		if err := os.Setenv(kv[0], kv[1]); err != nil && envErr == nil {
			envErr = fmt.Errorf("set %s: %w", kv[0], err)
		}
	}

	notifyWinInet()
	return envErr
}

func clearSystemProxyPlatform() error {
	// Cleanup must be idempotent: "value does not exist" is a success, not an
	// error. Real failures (access denied, hive missing) are surfaced because
	// leaving the proxy pointed at a dead core would cut the user's internet.
	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// 1. Update Internet Settings
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		record(fmt.Errorf("open internet settings registry key: %w", err))
	} else {
		record(k.SetDWordValue("ProxyEnable", 0))
		record(k.SetStringValue("ProxyServer", ""))
		// Deleting an absent value is a no-op success; ignore it deliberately.
		_ = k.DeleteValue("AutoConfigURL")
		_ = k.Close()
	}

	// 2. Update Connections binary blob to clear proxy flag
	ck, err := registry.OpenKey(registry.CURRENT_USER, connectionsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		record(fmt.Errorf("open connections registry key: %w", err))
	} else {
		blob := buildConnectionBlob(false, "", "")
		record(ck.SetBinaryValue("DefaultConnectionSettings", blob))
		record(ck.SetBinaryValue("SavedLegacySettings", blob))
		_ = ck.Close()
	}

	// 3. Clear environment variables (Unsetenv only fails on an invalid name).
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy", "NO_PROXY", "no_proxy"} {
		record(os.Unsetenv(name))
	}

	notifyWinInet()
	return firstErr
}

func buildConnectionBlob(enableProxy bool, proxyServer string, bypass string) []byte {
	var blob []byte
	blob = append(blob, 0x46, 0x00, 0x00, 0x00)
	blob = append(blob, 0x02, 0x00, 0x00, 0x00)
	if enableProxy {
		blob = append(blob, 0x03, 0x00, 0x00, 0x00)
	} else {
		blob = append(blob, 0x01, 0x00, 0x00, 0x00)
	}

	proxyBytes := []byte(proxyServer)
	if !enableProxy {
		proxyBytes = nil
	}
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(proxyBytes)))
	blob = append(blob, lenBuf...)
	blob = append(blob, proxyBytes...)

	bypassBytes := []byte(bypass)
	if !enableProxy {
		bypassBytes = nil
	}
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(bypassBytes)))
	blob = append(blob, lenBuf...)
	blob = append(blob, bypassBytes...)

	binary.LittleEndian.PutUint32(lenBuf, 0)
	blob = append(blob, lenBuf...)
	blob = append(blob, make([]byte, 32)...)

	return blob
}

// isStaleNexVPNProxy reports whether the OS-level system proxy currently points at one
// of NexVPN's own local proxy ports (127.0.0.1:108xx). Only in that case is it safe to
// assume the proxy setting was left behind by a previous NexVPN session rather than
// configured intentionally by the user or their network administrator.
func isStaleNexVPNProxy() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = k.Close() }()

	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return false
	}

	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return false
	}

	return strings.Contains(server, "127.0.0.1:108") || strings.Contains(server, "localhost:108")
}

// IsSystemProxyEnabled reports whether a system-wide WinINET proxy is currently
// enabled, regardless of which host it points at.
func IsSystemProxyEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = k.Close() }()

	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	return err == nil && enabled != 0
}

func notifyWinInet() {
	_, _, _ = procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	_, _, _ = procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
}
