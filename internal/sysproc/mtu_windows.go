//go:build windows

package sysproc

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Standard test payload sizes corresponding to MTU values (MTU - 28 bytes for IPv4 + ICMP header)
var mtuCandidates = []int{1500, 1492, 1460, 1420, 1380, 1280}

// isSafeProbeTarget reports whether host is safe to pass as an argument to
// ping.exe. Only literal IP addresses and syntactically valid hostnames are
// accepted: this prevents argument injection through a subscription-controlled
// server field (e.g. a value starting with "-" would otherwise be parsed as a
// ping flag).
func isSafeProbeTarget(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return false
	}
	if strings.HasPrefix(host, "-") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, r := range host {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum && r != '.' && r != '-' && r != '_' {
			return false
		}
	}
	// Must contain at least one dot to be a plausible FQDN, and must not start
	// or end with a dot/hyphen.
	if !strings.Contains(host, ".") {
		return false
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") ||
		strings.HasPrefix(host, "-") || strings.HasSuffix(host, "-") {
		return false
	}
	return true
}

// DiscoverOptimalMTU probes the target path with the Don't Fragment (DF) bit to discover the Path MTU
func DiscoverOptimalMTU(targetHost string) int {
	targetHost = strings.TrimSpace(targetHost)
	if !isSafeProbeTarget(targetHost) {
		targetHost = "1.1.1.1"
	}

	for _, mtu := range mtuCandidates {
		payloadSize := mtu - 28
		if payloadSize < 64 {
			continue
		}

		cmd := exec.Command("ping.exe", "-n", "1", "-w", "750", "-f", "-l", fmt.Sprintf("%d", payloadSize), targetHost)
		cmd.SysProcAttr = HideWindowAttr()

		out, err := cmd.CombinedOutput()
		outStr := strings.ToLower(string(out))

		// If ping succeeded and does not contain fragmentation errors
		if err == nil && !strings.Contains(outStr, "fragment") && !strings.Contains(outStr, "needs to be fragmented") {
			return mtu
		}
	}

	// Default fallback safe MTU for cellular/PPPoE tunnels
	return 1380
}

// ApplyWintunMTU sets the MTU on the specified network adapter subinterface without displaying console windows
func ApplyWintunMTU(adapterName string, mtu int) error {
	if adapterName == "" {
		adapterName = "NexVPN-TUN"
	}
	if mtu < 1280 || mtu > 1500 {
		mtu = 1380
	}

	// Retry applying MTU because adapter initialization in Windows may take a few milliseconds
	var lastErr error
	for i := 0; i < 3; i++ {
		cmdIPv4 := exec.Command("netsh", "interface", "ipv4", "set", "subinterface", adapterName, fmt.Sprintf("mtu=%d", mtu), "store=active")
		cmdIPv4.SysProcAttr = HideWindowAttr()
		if err := cmdIPv4.Run(); err == nil {
			// Also apply to IPv6 if present
			cmdIPv6 := exec.Command("netsh", "interface", "ipv6", "set", "subinterface", adapterName, fmt.Sprintf("mtu=%d", mtu), "store=active")
			cmdIPv6.SysProcAttr = HideWindowAttr()
			_ = cmdIPv6.Run()
			return nil
		} else {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
		}
	}

	return lastErr
}
