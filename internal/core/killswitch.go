package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"nexvpn/internal/sysproc"
)

type KillSwitchManager struct {
	mu     sync.Mutex
	active bool
}

var globalKillSwitch = &KillSwitchManager{}

func GetKillSwitch() *KillSwitchManager {
	return globalKillSwitch
}

func prepareHiddenCmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	return cmd
}

// Enable activates Windows Firewall blocking rules to prevent IP leaks while allowing VPN cores
func (k *KillSwitchManager) Enable(serverIP string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if runtime.GOOS != "windows" || k.active {
		return nil
	}

	// Remove any previous stale rules first
	k.disableUnlocked()

	// 1. Allow Loopback (127.0.0.1)
	cmdLoop := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=NexVPN-KillSwitch-Allow-Loopback", "dir=out", "action=allow", "remoteip=127.0.0.1,::1", "profile=any")
	_ = prepareHiddenCmd(cmdLoop).Run()

	// 2. Allow Private LAN
	cmdLAN := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=NexVPN-KillSwitch-Allow-LAN", "dir=out", "action=allow",
		"remoteip=192.168.0.0/16,10.0.0.0/8,172.16.0.0/12", "profile=any")
	_ = prepareHiddenCmd(cmdLAN).Run()

	// 3. Allow VPN Remote Server IP if available (resolve domain to IP if needed)
	if serverIP != "" {
		// Fix #6: netsh advfirewall needs an IP, not a domain name
		resolvedIP := resolveToIP(serverIP)
		if resolvedIP != "" {
			cmdServer := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
				"name=NexVPN-KillSwitch-Allow-Server", "dir=out", "action=allow",
				fmt.Sprintf("remoteip=%s", resolvedIP), "profile=any")
			_ = prepareHiddenCmd(cmdServer).Run()
		}
	}

	// 4. Allow Core Binaries
	execDir, _ := os.Executable()
	binDir := filepath.Join(filepath.Dir(execDir), "bin")
	for _, binName := range []string{"sing-box.exe", "xray.exe", "openvpn.exe"} {
		binPath := filepath.Join(binDir, binName)
		if _, err := os.Stat(binPath); err == nil {
			cmdBin := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
				fmt.Sprintf("name=NexVPN-KillSwitch-Allow-%s", binName), "dir=out", "action=allow",
				fmt.Sprintf("program=%s", binPath), "profile=any")
			_ = prepareHiddenCmd(cmdBin).Run()
		}
	}

	// 5. Block all remaining outbound internet traffic
	cmdBlock := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=NexVPN-KillSwitch", "dir=out", "action=block", "protocol=ANY", "profile=any")
	_ = prepareHiddenCmd(cmdBlock).Run()

	k.active = true
	return nil
}

// Disable removes the Windows Firewall kill switch rules
func (k *KillSwitchManager) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.disableUnlocked()
}

func (k *KillSwitchManager) disableUnlocked() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	for _, ruleName := range []string{
		"NexVPN-KillSwitch",
		"NexVPN-KillSwitch-Allow-Loopback",
		"NexVPN-KillSwitch-Allow-LAN",
		"NexVPN-KillSwitch-Allow-Server",
		"NexVPN-KillSwitch-Allow-sing-box.exe",
		"NexVPN-KillSwitch-Allow-xray.exe",
		"NexVPN-KillSwitch-Allow-openvpn.exe",
	} {
		cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", ruleName))
		_ = prepareHiddenCmd(cmd).Run()
	}

	k.active = false
	return nil
}

func (k *KillSwitchManager) IsActive() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.active
}

// resolveToIP returns a valid IP string from the input.
// If the input is already an IP, it returns it directly.
// If it is a hostname, it performs a DNS lookup and returns the first IPv4 result.
func resolveToIP(hostOrIP string) string {
	if hostOrIP == "" {
		return ""
	}
	// Already an IP address?
	if ip := net.ParseIP(hostOrIP); ip != nil {
		return hostOrIP
	}
	// Domain: try DNS lookup
	addrs, err := net.LookupHost(hostOrIP)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	// Prefer IPv4
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			return a
		}
	}
	return addrs[0]
}

