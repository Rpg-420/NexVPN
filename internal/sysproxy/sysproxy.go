package sysproxy

import (
	"encoding/binary"
	"fmt"
	"runtime"
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
const defaultBypass = `localhost;127.*;10.*;172.16.*;192.168.*;*.ir;<local>`

// SetSystemProxy enables Windows system proxy pointing to the given host:port.
// Deprecated: Use SetFullSystemProxy for dual HTTP/SOCKS configuration.
// The host parameter is currently always resolved to 127.0.0.1; use SetFullSystemProxy for custom hosts.
func SetSystemProxy(host string, port int) error {
	if host == "" {
		host = "127.0.0.1"
	}
	// For custom hosts (e.g. LAN sharing), set the proxy string directly
	if host != "127.0.0.1" && host != "localhost" {
		proxyAddr := fmt.Sprintf("http=%s:%d;https=%s:%d;socks=%s:%d", host, port, host, port, host, port)
		k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
		if err == nil {
			_ = k.SetDWordValue("ProxyEnable", 1)
			_ = k.SetStringValue("ProxyServer", proxyAddr)
			_ = k.SetStringValue("ProxyOverride", defaultBypass)
			k.Close()
		}
		notifyWinInet()
		return nil
	}
	return SetFullSystemProxy(port, 10808)
}

// SetFullSystemProxy configures HTTP and SOCKS proxy in Windows Internet Settings
func SetFullSystemProxy(httpPort int, socksPort int) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	if httpPort <= 0 {
		httpPort = 10809
	}
	if socksPort <= 0 {
		socksPort = 10808
	}

	// Windows standard proxy format: "http=127.0.0.1:10809;https=127.0.0.1:10809;socks=127.0.0.1:10808"
	proxyAddr := fmt.Sprintf("http=127.0.0.1:%d;https=127.0.0.1:%d;socks=127.0.0.1:%d", httpPort, httpPort, socksPort)

	// 1. Update Internet Settings Registry Key
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err == nil {
		_ = k.SetDWordValue("ProxyEnable", 1)
		_ = k.SetStringValue("ProxyServer", proxyAddr)
		_ = k.SetStringValue("ProxyOverride", defaultBypass)
		_ = k.DeleteValue("AutoConfigURL")
		k.Close()
	}

	// 2. Update Connections binary blob (DefaultConnectionSettings & SavedLegacySettings)
	ck, err := registry.OpenKey(registry.CURRENT_USER, connectionsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err == nil {
		blob := buildConnectionBlob(true, proxyAddr, defaultBypass)
		_ = ck.SetBinaryValue("DefaultConnectionSettings", blob)
		_ = ck.SetBinaryValue("SavedLegacySettings", blob)
		ck.Close()
	}

	// 3. Set environment variables for shell tools and CLI processes
	httpProxyURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	socksProxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	_ = syscall.Setenv("HTTP_PROXY", httpProxyURL)
	_ = syscall.Setenv("HTTPS_PROXY", httpProxyURL)
	_ = syscall.Setenv("ALL_PROXY", socksProxyURL)
	_ = syscall.Setenv("http_proxy", httpProxyURL)
	_ = syscall.Setenv("https_proxy", httpProxyURL)
	_ = syscall.Setenv("all_proxy", socksProxyURL)
	_ = syscall.Setenv("NO_PROXY", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	_ = syscall.Setenv("no_proxy", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

	notifyWinInet()
	return nil
}

// ClearSystemProxy disables Windows system proxy completely
func ClearSystemProxy() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	// 1. Update Internet Settings
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err == nil {
		_ = k.SetDWordValue("ProxyEnable", 0)
		_ = k.SetStringValue("ProxyServer", "")
		_ = k.DeleteValue("AutoConfigURL")
		k.Close()
	}

	// 2. Update Connections binary blob to clear proxy flag
	ck, err := registry.OpenKey(registry.CURRENT_USER, connectionsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err == nil {
		blob := buildConnectionBlob(false, "", "")
		_ = ck.SetBinaryValue("DefaultConnectionSettings", blob)
		_ = ck.SetBinaryValue("SavedLegacySettings", blob)
		ck.Close()
	}

	// 3. Clear environment variables
	_ = syscall.Unsetenv("HTTP_PROXY")
	_ = syscall.Unsetenv("HTTPS_PROXY")
	_ = syscall.Unsetenv("ALL_PROXY")
	_ = syscall.Unsetenv("http_proxy")
	_ = syscall.Unsetenv("https_proxy")
	_ = syscall.Unsetenv("all_proxy")
	_ = syscall.Unsetenv("NO_PROXY")
	_ = syscall.Unsetenv("no_proxy")

	notifyWinInet()
	return nil
}

// CheckAndClearStaleProxy resets system proxy if it was left pointing to local NexVPN ports
func CheckAndClearStaleProxy() {
	if runtime.GOOS != "windows" {
		return
	}
	_ = ClearSystemProxy()
}

// buildConnectionBlob generates a complete, compliant WinINet Connections binary structure
func buildConnectionBlob(enableProxy bool, proxyServer string, bypass string) []byte {
	var blob []byte

	// Header: 0x46 (70), 0, 0, 0
	blob = append(blob, 0x46, 0x00, 0x00, 0x00)

	// Counter: 4 bytes (e.g. 0x02, 0, 0, 0)
	blob = append(blob, 0x02, 0x00, 0x00, 0x00)

	// Flags: 4 bytes (0x03 for proxy enabled, 0x01 for direct)
	if enableProxy {
		blob = append(blob, 0x03, 0x00, 0x00, 0x00)
	} else {
		blob = append(blob, 0x01, 0x00, 0x00, 0x00)
	}

	// Proxy Server String
	proxyBytes := []byte(proxyServer)
	if !enableProxy {
		proxyBytes = nil
	}
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(proxyBytes)))
	blob = append(blob, lenBuf...)
	blob = append(blob, proxyBytes...)

	// Bypass / Override String
	bypassBytes := []byte(bypass)
	if !enableProxy {
		bypassBytes = nil
	}
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(bypassBytes)))
	blob = append(blob, lenBuf...)
	blob = append(blob, bypassBytes...)

	// AutoConfigURL (empty)
	binary.LittleEndian.PutUint32(lenBuf, 0)
	blob = append(blob, lenBuf...)

	// Trailing 32 bytes
	blob = append(blob, make([]byte, 32)...)

	return blob
}

func notifyWinInet() {
	if runtime.GOOS != "windows" {
		return
	}
	procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
}
