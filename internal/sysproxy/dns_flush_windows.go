//go:build windows

package sysproxy

import (
	"os/exec"
	"syscall"
)

var (
	modDnsApi                 = syscall.NewLazyDLL("dnsapi.dll")
	procDnsFlushResolverCache = modDnsApi.NewProc("DnsFlushResolverCache")
)

func flushDNSPlatform() error {
	if procDnsFlushResolverCache.Find() == nil {
		r1, _, _ := procDnsFlushResolverCache.Call()
		if r1 != 0 {
			return nil
		}
	}
	// Fallback to CLI command if API call fails or not available
	cmd := exec.Command("ipconfig", "/flushdns")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
