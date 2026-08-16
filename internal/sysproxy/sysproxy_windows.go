//go:build windows

package sysproxy

import (
	"encoding/binary"
	"fmt"
	"os"
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

func setSystemProxyPlatform(host string, port int) error {
	if host == "" {
		host = "127.0.0.1"
	}
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
	return setFullSystemProxyPlatform(port, 10808)
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
	_ = os.Setenv("HTTP_PROXY", httpProxyURL)
	_ = os.Setenv("HTTPS_PROXY", httpProxyURL)
	_ = os.Setenv("ALL_PROXY", socksProxyURL)
	_ = os.Setenv("http_proxy", httpProxyURL)
	_ = os.Setenv("https_proxy", httpProxyURL)
	_ = os.Setenv("all_proxy", socksProxyURL)
	_ = os.Setenv("NO_PROXY", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	_ = os.Setenv("no_proxy", "localhost,127.0.0.1,*.ir,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

	notifyWinInet()
	return nil
}

func clearSystemProxyPlatform() error {
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
	_ = os.Unsetenv("HTTP_PROXY")
	_ = os.Unsetenv("HTTPS_PROXY")
	_ = os.Unsetenv("ALL_PROXY")
	_ = os.Unsetenv("http_proxy")
	_ = os.Unsetenv("https_proxy")
	_ = os.Unsetenv("all_proxy")
	_ = os.Unsetenv("NO_PROXY")
	_ = os.Unsetenv("no_proxy")

	notifyWinInet()
	return nil
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

func notifyWinInet() {
	procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	procInternetSetOption.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
}
