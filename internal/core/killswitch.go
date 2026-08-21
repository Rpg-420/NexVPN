package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// Enable activates Firewall blocking rules to prevent IP leaks while allowing VPN cores
func (k *KillSwitchManager) Enable(serverIP string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.active {
		return nil
	}

	// Remove any previous stale rules first
	k.disableUnlocked()

	if runtime.GOOS == "windows" {
		// 1. Allow Loopback (127.0.0.1)
		cmdLoop := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name=NexVPN-KillSwitch-Allow-Loopback", "dir=out", "action=allow", "remoteip=127.0.0.1,::1", "profile=any")
		_ = prepareHiddenCmd(cmdLoop).Run()

		// 2. Allow Private LAN
		cmdLAN := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name=NexVPN-KillSwitch-Allow-LAN", "dir=out", "action=allow",
			"remoteip=192.168.0.0/16,10.0.0.0/8,172.16.0.0/12", "profile=any")
		_ = prepareHiddenCmd(cmdLAN).Run()

		// 3. Allow VPN Remote Server IP if available
		if serverIP != "" {
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

	} else if runtime.GOOS == "linux" {
		// Linux iptables Kill Switch
		_ = runCmd("iptables", "-N", "NEXVPN-KILLSWITCH")
		_ = runCmd("iptables", "-I", "OUTPUT", "1", "-j", "NEXVPN-KILLSWITCH")

		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "lo", "-j", "ACCEPT")
		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "tun+", "-j", "ACCEPT")
		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "NexVPN-TUN", "-j", "ACCEPT")
		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "192.168.0.0/16", "-j", "ACCEPT")
		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "10.0.0.0/8", "-j", "ACCEPT")
		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "172.16.0.0/12", "-j", "ACCEPT")

		if serverIP != "" {
			resolvedIP := resolveToIP(serverIP)
			if resolvedIP != "" {
				_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", resolvedIP, "-j", "ACCEPT")
			}
		}

		_ = runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-j", "REJECT", "--reject-with", "icmp-net-prohibited")

	} else if runtime.GOOS == "darwin" {
		// macOS pfctl Kill Switch
		resolvedIP := ""
		if serverIP != "" {
			resolvedIP = resolveToIP(serverIP)
		}

		// Create a rule string
		rules := "block out all\npass out on lo0 to any\npass out on utun+ to any\npass out to 192.168.0.0/16\npass out to 10.0.0.0/8\npass out to 172.16.0.0/12\n"
		if resolvedIP != "" {
			rules += fmt.Sprintf("pass out to %s\n", resolvedIP)
		}

		// Write rules to a temporary file
		tmpFile := "/tmp/nexvpn-pf.conf"
		_ = os.WriteFile(tmpFile, []byte(rules), 0644)

		// Enable pf and load rules
		_ = runCmd("pfctl", "-E")
		_ = runCmd("pfctl", "-a", "nexvpn-killswitch", "-f", tmpFile)
	}

	k.active = true
	return nil
}

// Disable removes the Firewall kill switch rules
func (k *KillSwitchManager) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.disableUnlocked()
}

func (k *KillSwitchManager) disableUnlocked() error {
	if !k.active {
		return nil
	}

	if runtime.GOOS == "windows" {
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
	} else if runtime.GOOS == "linux" {
		// Remove iptables rules
		_ = runCmd("iptables", "-D", "OUTPUT", "-j", "NEXVPN-KILLSWITCH")
		_ = runCmd("iptables", "-F", "NEXVPN-KILLSWITCH")
		_ = runCmd("iptables", "-X", "NEXVPN-KILLSWITCH")
	} else if runtime.GOOS == "darwin" {
		// Flush macOS pfctl rules
		_ = runCmd("pfctl", "-a", "nexvpn-killswitch", "-F", "all")
	}

	k.active = false
	return nil
}

func runCmd(prog string, args ...string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		pkArgs := append([]string{prog}, args...)
		cmd = exec.Command("pkexec", pkArgs...)
	} else if runtime.GOOS == "darwin" && os.Geteuid() != 0 {
		cmdStr := fmt.Sprintf("%s %s", prog, strings.Join(args, " "))
		appleScript := fmt.Sprintf("do shell script %q with administrator privileges", cmdStr)
		cmd = exec.Command("osascript", "-e", appleScript)
	} else {
		cmd = exec.Command(prog, args...)
	}

	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = sysproc.HideWindowAttr()
	}

	return cmd.Run()
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
