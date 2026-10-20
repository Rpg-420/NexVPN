package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
)

type KillSwitchManager struct {
	mu     sync.Mutex
	active bool
	// swept records that this process has already removed any rule set left
	// behind by an earlier (possibly crashed) run. Until it is set, the first
	// Disable/Enable must do a full teardown; afterwards the teardown is pure
	// cost, because this process knows it installed nothing.
	swept bool
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

	// Sweep any rule set left behind by an earlier run. Skipped once the session
	// has already proven the machine clean: `Connect` calls Disable() before
	// every attempt, and that first call is the one that pays for this.
	if !k.swept {
		k.swept = true
		k.disableUnlocked()
	}

	// Resolve the VPN server *before* the block policy is installed.
	//
	// The previous order applied `blockoutbound` first and only then resolved
	// the server name, which meant the DNS lookup ran against a machine that was
	// already firewalled — including against its own resolver. The lookup
	// routinely failed, the allow-rule for the server was silently skipped (its
	// error was not even recorded in firstErr), and the result was a kill switch
	// that reported success while blocking the VPN's own traffic. Resolving
	// first removes the dependency: nothing is blocked yet, so DNS still works.
	var serverIPs []string
	if serverIP != "" {
		serverIPs = resolveAllIPs(serverIP)
		if len(serverIPs) == 0 {
			diary.Log("killswitch", "resolve-failed", "server", serverIP)
		} else {
			diary.Log("killswitch", "resolve-ok", "server", serverIP, "count", strconv.Itoa(len(serverIPs)))
		}
	}

	var firstErr error
	check := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if runtime.GOOS == "windows" {
		// 1. Set Windows Firewall default outbound policy to block.
		//    The allow rules below override this default. No explicit "block all"
		//    rule is added - see the note at the end of this block.
		cmdPolicy := exec.Command("netsh", "advfirewall", "set", "allprofiles", "firewallpolicy", "blockoutbound,allowinbound")
		check(prepareHiddenCmd(cmdPolicy).Run())

		// 2. Allow Loopback (127.0.0.1, ::1)
		cmdLoop := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name=NexVPN-KillSwitch-Allow-Loopback", "dir=out", "action=allow", "remoteip=127.0.0.1,::1", "profile=any")
		check(prepareHiddenCmd(cmdLoop).Run())

		// 3. Allow Private LAN
		cmdLAN := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name=NexVPN-KillSwitch-Allow-LAN", "dir=out", "action=allow",
			"remoteip=192.168.0.0/16,10.0.0.0/8,172.16.0.0/12", "profile=any")
		check(prepareHiddenCmd(cmdLAN).Run())

		// 4. Allow Bootstrap DNS Resolvers (1.1.1.1, 8.8.8.8, 1.0.0.1, 8.8.4.4)
		cmdDNS := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
			"name=NexVPN-KillSwitch-Allow-DNS", "dir=out", "action=allow",
			"remoteip=1.1.1.1,8.8.8.8,1.0.0.1,8.8.4.4", "profile=any")
		check(prepareHiddenCmd(cmdDNS).Run())

		// 5. Allow VPN Remote Server IPs. The addresses were resolved before the
		//    block policy went in (see the top of this function) because DNS no
		//    longer works once outbound traffic is blocked.
		if len(serverIPs) > 0 {
			ipListStr := strings.Join(serverIPs, ",")
			cmdServer := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
				"name=NexVPN-KillSwitch-Allow-Server", "dir=out", "action=allow",
				fmt.Sprintf("remoteip=%s", ipListStr), "profile=any")
			check(prepareHiddenCmd(cmdServer).Run())
		}

		// 6. Allow Core Binaries
		execDir, _ := os.Executable()
		binDir := filepath.Join(filepath.Dir(execDir), "bin")
		for _, binName := range []string{
			"sing-box.exe",
			"xray.exe",
			"openvpn.exe",
			"aether.exe",
			"psiphon.exe",
			"psiphon-tunnel-core.exe",
		} {
			binPath := filepath.Join(binDir, binName)
			if _, err := os.Stat(binPath); err == nil {
				cmdBin := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
					fmt.Sprintf("name=NexVPN-KillSwitch-Allow-%s", binName), "dir=out", "action=allow",
					fmt.Sprintf("program=%s", binPath), "profile=any")
				check(prepareHiddenCmd(cmdBin).Run())
			}
		}

		// NOTE: no explicit "block all outbound" rule is added here on purpose.
		// The default outbound policy was already switched to "block" in step 1, and
		// Windows Defender Firewall gives *block* rules precedence over *allow* rules.
		// Adding an additional `action=block protocol=ANY` rule would therefore
		// blackhole the VPN core's own traffic (including the remote server IP that
		// was explicitly allowed above), leaving the user with no connectivity at all.
		// The default-block policy + the allow rules above are sufficient and correct.
	} else if runtime.GOOS == "linux" {
		// Linux iptables Kill Switch
		check(runCmd("iptables", "-N", "NEXVPN-KILLSWITCH"))
		check(runCmd("iptables", "-I", "OUTPUT", "1", "-j", "NEXVPN-KILLSWITCH"))

		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "lo", "-j", "ACCEPT"))
		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "tun+", "-j", "ACCEPT"))
		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-o", "NexVPN-TUN", "-j", "ACCEPT"))
		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "192.168.0.0/16", "-j", "ACCEPT"))
		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "10.0.0.0/8", "-j", "ACCEPT"))
		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", "172.16.0.0/12", "-j", "ACCEPT"))

		for _, ip := range serverIPs {
			check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-d", ip, "-j", "ACCEPT"))
		}

		check(runCmd("iptables", "-A", "NEXVPN-KILLSWITCH", "-j", "REJECT", "--reject-with", "icmp-net-prohibited"))

	} else if runtime.GOOS == "darwin" {
		// macOS pfctl Kill Switch
		// Create a rule string
		rules := "block out all\npass out on lo0 to any\npass out on utun+ to any\npass out to 192.168.0.0/16\npass out to 10.0.0.0/8\npass out to 172.16.0.0/12\n"
		for _, ip := range serverIPs {
			rules += fmt.Sprintf("pass out to %s\n", ip)
		}

		// Write rules to a temporary file
		tmpFile := "/tmp/nexvpn-pf.conf"
		_ = os.WriteFile(tmpFile, []byte(rules), 0600)

		// Enable pf and load rules
		check(runCmd("pfctl", "-E"))
		check(runCmd("pfctl", "-a", "nexvpn-killswitch", "-f", tmpFile))
	}

	if firstErr != nil {
		k.disableUnlocked()
		return fmt.Errorf("failed to configure kill switch: %w", firstErr)
	}

	// Everything is in place except, possibly, the rule that lets the VPN's own
	// traffic out. Reporting success there produces exactly the state the user
	// cannot diagnose from the UI — "connected", kill switch on, and no
	// connectivity at all — so roll the whole thing back and say why instead.
	// A server given as a literal IP always resolves, so this only fires for a
	// hostname that the system resolver could not answer.
	if serverIP != "" && len(serverIPs) == 0 {
		k.disableUnlocked()
		return fmt.Errorf(
			"kill switch aborted: could not resolve the VPN server address %q, so its allow rule could not be created",
			serverIP)
	}

	k.active = true
	return nil
}

// Disable removes the Firewall kill switch rules.
//
// When this process installed nothing and has already proven the machine clean,
// this is a no-op. That distinction is what keeps `Connect` fast: the previous
// implementation ran the full teardown unconditionally, which costs one `netsh`
// process for the policy plus eleven more for the rule deletions, before *every*
// connection attempt — seconds of dead time on the path the user is waiting on,
// spent deleting rules that did not exist.
func (k *KillSwitchManager) Disable() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.active && k.swept {
		return nil
	}
	k.swept = true
	return k.disableUnlocked()
}

// ForceCleanup removes the kill switch rules regardless of the manager's belief
// about its own state. Used on paths that must repair the machine after an
// abnormal exit — a force-killed process can leave the global outbound policy
// set to block, which takes the whole machine offline until it is reset.
func (k *KillSwitchManager) ForceCleanup() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.swept = true
	return k.disableUnlocked()
}

func (k *KillSwitchManager) disableUnlocked() error {
	// Always attempt to remove stale firewall rules even if active==false
	// (e.g. after crash, startup cleanup must succeed).

	if runtime.GOOS == "windows" {
		// Restore default Windows Firewall policy (allow outbound)
		cmdRestorePolicy := exec.Command("netsh", "advfirewall", "set", "allprofiles", "firewallpolicy", "allowoutbound,blockinbound")
		_ = prepareHiddenCmd(cmdRestorePolicy).Run()

		for _, ruleName := range []string{
			// "NexVPN-KillSwitch" is the legacy block-all rule created by older builds.
			// It is no longer created (see Enable), but is still deleted here so that
			// upgrading users get the old, connectivity-breaking rule removed.
			"NexVPN-KillSwitch",
			"NexVPN-KillSwitch-Allow-Loopback",
			"NexVPN-KillSwitch-Allow-LAN",
			"NexVPN-KillSwitch-Allow-DNS",
			"NexVPN-KillSwitch-Allow-Server",
			"NexVPN-KillSwitch-Allow-sing-box.exe",
			"NexVPN-KillSwitch-Allow-xray.exe",
			"NexVPN-KillSwitch-Allow-openvpn.exe",
			"NexVPN-KillSwitch-Allow-aether.exe",
			"NexVPN-KillSwitch-Allow-psiphon.exe",
			"NexVPN-KillSwitch-Allow-psiphon-tunnel-core.exe",
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

// containsUnsafeShellChars reports whether s contains control characters or shell
// metacharacters that could break out of an argument when a command is assembled
// into a shell string (the macOS `osascript` path does exactly that).
//
// NOTE: the previous implementation tested against the Go literal ";&|$`\\n\\r\\t".
// In a Go double-quoted string `\\n` is NOT an escape sequence: it denotes the two
// literal characters '\' and 'n'. The set actually used was therefore
// {; & | $ ` \ n r t} — which rejected any argument containing the letters n, r or
// t (breaking e.g. "--reject-with icmp-net-prohibited" and "-o tun+") while failing
// to reject real newlines and tabs. Both problems are fixed below.
func containsUnsafeShellChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f { // control characters incl. real \n \r \t
			return true
		}
		switch r {
		case ';', '&', '|', '$', '`', '\\', '"', '\'':
			return true
		}
	}
	return false
}

func runCmd(prog string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Validate program name to prevent command injection
	if prog == "" || containsUnsafeShellChars(prog) {
		return fmt.Errorf("invalid program name: %q", prog)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		// Validate arguments to prevent command injection
		for _, arg := range args {
			if containsUnsafeShellChars(arg) {
				return fmt.Errorf("invalid argument: %q", arg)
			}
		}
		pkArgs := append([]string{prog}, args...)
		cmd = exec.CommandContext(ctx, "pkexec", pkArgs...)
	} else if runtime.GOOS == "darwin" && os.Geteuid() != 0 {
		var sanitizedArgs []string
		for _, a := range args {
			// Validate argument to prevent command injection
			if containsUnsafeShellChars(a) {
				return fmt.Errorf("invalid argument: %q", a)
			}
			sanitizedArgs = append(sanitizedArgs, fmt.Sprintf("%q", a))
		}
		cmdStr := fmt.Sprintf("%s %s", prog, strings.Join(sanitizedArgs, " "))
		cmdStr = strings.ReplaceAll(cmdStr, `\`, `\\`)
		cmdStr = strings.ReplaceAll(cmdStr, `"`, `\"`)
		appleScript := fmt.Sprintf(`do shell script "%s" with administrator privileges`, cmdStr)
		cmd = exec.CommandContext(ctx, "osascript", "-e", appleScript)
	} else {
		// Validate arguments to prevent command injection
		for _, arg := range args {
			if containsUnsafeShellChars(arg) {
				return fmt.Errorf("invalid argument: %q", arg)
			}
		}
		cmd = exec.CommandContext(ctx, prog, args...)
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

// resolveAllIPs returns all valid IP strings from the input host or IP.
// If the input is already an IP, it returns it directly.
// If it is a hostname, it performs a DNS lookup and returns all IPv4/IPv6 results.
func resolveAllIPs(hostOrIP string) []string {
	if hostOrIP == "" {
		return nil
	}
	// Already an IP address?
	if ip := net.ParseIP(hostOrIP); ip != nil {
		return []string{hostOrIP}
	}
	// Domain: try DNS lookup with timeout
	var netResolver net.Resolver
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := netResolver.LookupHost(ctx, hostOrIP)
	if err != nil || len(addrs) == 0 {
		return nil
	}
	var res []string
	seen := make(map[string]bool)
	for _, a := range addrs {
		if !seen[a] && net.ParseIP(a) != nil {
			seen[a] = true
			res = append(res, a)
		}
	}
	return res
}
