package parser

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

	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"
)

func hiddenCmd(prog string, args ...string) *exec.Cmd {
	cmd := exec.Command(prog, args...)
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	return cmd
}

type DiagnosticItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"` // "ok", "warning", "error", "running"
	LatencyMs   int    `json:"latency_ms"`
	Detail      string `json:"detail"`
}

type DoctorReport struct {
	OverallStatus string           `json:"overall_status"` // "healthy", "degraded", "critical"
	Summary       string           `json:"summary"`
	Items         []DiagnosticItem `json:"items"`
	Timestamp     int64            `json:"timestamp"`
}

type RepairResult struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Actions []string `json:"actions"`
}

// RunNetworkDoctor performs comprehensive automated network & system diagnosis
func RunNetworkDoctor(ctx context.Context) DoctorReport {
	report := DoctorReport{
		Timestamp: time.Now().Unix(),
		Items:     make([]DiagnosticItem, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	addItem := func(item DiagnosticItem) {
		mu.Lock()
		defer mu.Unlock()
		report.Items = append(report.Items, item)
	}

	// 1. Check Local Gateway / Default Route
	wg.Add(1)
	go func() {
		defer wg.Done()
		item := DiagnosticItem{
			ID:          "gateway",
			Title:       "Local Gateway & Router Connection",
			Description: "Detects the real default gateway and probes its reachability",
			Status:      "ok",
		}
		gw := defaultGateway()
		if gw == "" {
			item.Status = "warning"
			item.Detail = "No IPv4 default gateway detected on this machine"
			addItem(item)
			return
		}
		start := time.Now()
		// Routers commonly answer on 53/tcp (DNS) or 80/tcp (admin page). A
		// non-responding gateway is reported as a warning rather than a silent
		// "ok": the old code claimed success even when the probe failed, which
		// masked genuine LAN outages.
		reachable := checkPort(gw, 53, 700*time.Millisecond) || checkPort(gw, 80, 700*time.Millisecond)
		dur := int(time.Since(start).Milliseconds())
		if reachable {
			item.Status = "ok"
			item.LatencyMs = dur
			item.Detail = fmt.Sprintf("Gateway %s responded in %d ms", gw, dur)
		} else {
			item.Status = "warning"
			item.Detail = fmt.Sprintf("Gateway %s did not answer on TCP 53/80 (may be filtered by the router)", gw)
		}
		addItem(item)
	}()

	// 2. Check Internet Connectivity & DNS Reachability
	wg.Add(1)
	go func() {
		defer wg.Done()
		item := DiagnosticItem{
			ID:          "dns_servers",
			Title:       "Public DNS Resolvers Health",
			Description: "Testing remote reachability to 1.1.1.1 and 8.8.8.8",
			Status:      "ok",
		}
		start := time.Now()
		conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 1000*time.Millisecond)
		dur := int(time.Since(start).Milliseconds())
		if err == nil {
			_ = conn.Close()
			item.Status = "ok"
			item.LatencyMs = dur
			item.Detail = fmt.Sprintf("Cloudflare DNS responding in %d ms", dur)
		} else {
			// Fallback check 8.8.8.8
			conn2, err2 := net.DialTimeout("tcp", "8.8.8.8:53", 1000*time.Millisecond)
			if err2 == nil {
				_ = conn2.Close()
				item.Status = "ok"
				item.Detail = "Google DNS is reachable"
			} else {
				item.Status = "warning"
				item.Detail = "DNS UDP port might be throttled on local ISP"
			}
		}
		addItem(item)
	}()

	// 3. Check Cloudflare Edge Reachability
	wg.Add(1)
	go func() {
		defer wg.Done()
		item := DiagnosticItem{
			ID:          "warp_edge",
			Title:       "Cloudflare Anycast Network Edge",
			Description: "Verifying open routes for WARP and WireGuard tunnels",
			Status:      "ok",
		}
		start := time.Now()
		conn, err := net.DialTimeout("tcp", "162.159.192.1:443", 1000*time.Millisecond)
		dur := int(time.Since(start).Milliseconds())
		if err == nil {
			_ = conn.Close()
			item.Status = "ok"
			item.LatencyMs = dur
			item.Detail = fmt.Sprintf("Anycast edge reachable with %d ms latency", dur)
		} else {
			item.Status = "warning"
			item.Detail = "Clean IP scanning or port adjustment recommended"
		}
		addItem(item)
	}()

	// 4. Check System Proxy Status
	wg.Add(1)
	go func() {
		defer wg.Done()
		item := DiagnosticItem{
			ID:          "system_proxy",
			Title:       "System Proxy Configuration State",
			Description: "Ensuring no stale or conflicting proxy settings remain active",
			Status:      "ok",
			Detail:      "System proxy settings are clean and normal",
		}
		// Single cross-platform implementation lives in the sysproxy package so
		// the doctor and the diagnostics report can never disagree about it.
		if sysproxy.IsSystemProxyEnabled() {
			item.Status = "warning"
			item.Detail = "System proxy is currently enabled (reset recommended if disconnected)"
		}
		addItem(item)
	}()

	// 5. Check Wintun Virtual Adapter Driver
	wg.Add(1)
	go func() {
		defer wg.Done()
		item := DiagnosticItem{
			ID:          "wintun_driver",
			Title:       "Virtual Network Adapter Driver (Wintun TUN)",
			Description: "Verifying the wintun.dll tunnel driver is actually present and usable",
			Status:      "ok",
		}
		switch runtime.GOOS {
		case "windows":
			// Probe the same locations the engine itself resolves at runtime
			// (see core.EnsureWintunDriver) so this check cannot disagree with
			// what the app will actually load.
			if path, size := findWintunDLL(); path != "" {
				item.Status = "ok"
				item.Detail = fmt.Sprintf("wintun.dll present (%d KB) at %s", size/1024, path)
			} else {
				item.Status = "error"
				item.Detail = "wintun.dll not found next to the executable or in bin/ — TUN mode will fail to start"
			}
		case "linux":
			if _, err := os.Stat("/dev/net/tun"); err == nil {
				item.Status = "ok"
				item.Detail = "/dev/net/tun is available"
			} else {
				item.Status = "error"
				item.Detail = "/dev/net/tun is missing — load the 'tun' kernel module or enable TUN in the container"
			}
		case "darwin":
			item.Status = "ok"
			item.Detail = "utun is provided natively by macOS"
		default:
			item.Status = "warning"
			item.Detail = "TUN support on this platform could not be verified"
		}
		addItem(item)
	}()

	wg.Wait()

	hasError := false
	hasWarning := false
	for _, it := range report.Items {
		if it.Status == "error" {
			hasError = true
		} else if it.Status == "warning" {
			hasWarning = true
		}
	}

	if hasError {
		report.OverallStatus = "critical"
		report.Summary = "Network issues detected; use 1-click auto-repair to fix."
	} else if hasWarning {
		report.OverallStatus = "degraded"
		report.Summary = "Network is operational, but optimizations are recommended."
	} else {
		report.OverallStatus = "healthy"
		report.Summary = "Network stack is 100% healthy, stable, and ready to connect."
	}

	return report
}

// RepairNetworkIssues executes automatic recovery and fixes network roadblocks.
// Success is derived from what actually ran: a repair is only reported as
// successful when every attempted step reported a real outcome instead of an
// error, so the UI can never show "repaired" for a no-op.
func RepairNetworkIssues() RepairResult {
	actions := make([]string, 0)
	failures := 0
	attempted := 0

	// 1. Terminate any orphaned core processes
	attempted++
	if runtime.GOOS == "windows" {
		_ = hiddenCmd("taskkill", "/F", "/IM", "sing-box.exe").Run()
		_ = hiddenCmd("taskkill", "/F", "/IM", "xray.exe").Run()
		_ = hiddenCmd("taskkill", "/F", "/IM", "openvpn.exe").Run()
		_ = hiddenCmd("taskkill", "/F", "/IM", "aether.exe").Run()
		_ = hiddenCmd("taskkill", "/F", "/IM", "psiphon.exe").Run()
		_ = hiddenCmd("taskkill", "/F", "/IM", "psiphon-tunnel-core.exe").Run()
		actions = append(actions, "Cleaned any orphaned core processes (sing-box, xray, openvpn, aether, psiphon)")
	} else {
		cleaned := 0
		for _, prog := range []string{"sing-box", "xray", "openvpn", "aether", "psiphon", "psiphon-tunnel-core"} {
			if err := exec.Command("pkill", "-9", "-x", prog).Run(); err == nil {
				cleaned++
			}
		}
		if cleaned > 0 {
			actions = append(actions, fmt.Sprintf("Cleaned %d orphaned core process group(s) (sing-box, xray, openvpn, aether, psiphon)", cleaned))
		} else {
			actions = append(actions, "No orphaned core processes were running")
		}
	}

	// 2. Flush DNS Cache — the outcome now reflects the real exit status
	attempted++
	dnsFlushed := false
	switch runtime.GOOS {
	case "windows":
		if err := hiddenCmd("ipconfig", "/flushdns").Run(); err == nil {
			dnsFlushed = true
			actions = append(actions, "Windows DNS cache successfully flushed (ipconfig /flushdns)")
		} else {
			failures++
			actions = append(actions, "Failed to flush Windows DNS cache (ipconfig /flushdns)")
		}
	case "darwin":
		errA := exec.Command("dscacheutil", "-flushcache").Run()
		errB := exec.Command("killall", "-HUP", "mDNSResponder").Run()
		if errA == nil || errB == nil {
			dnsFlushed = true
			actions = append(actions, "macOS DNS cache successfully flushed")
		} else {
			failures++
			actions = append(actions, "Failed to flush macOS DNS cache")
		}
	default:
		if err := exec.Command("resolvectl", "flush-caches").Run(); err == nil {
			dnsFlushed = true
			actions = append(actions, "Linux DNS cache successfully flushed (resolvectl flush-caches)")
		} else {
			failures++
			actions = append(actions, "Failed to flush Linux DNS cache (resolvectl flush-caches)")
		}
	}
	_ = dnsFlushed

	// 3. Clear System Proxy — report the genuine result
	attempted++
	if err := sysproxy.ClearSystemProxy(); err == nil {
		actions = append(actions, "System proxy settings disabled and cleared")
	} else {
		failures++
		actions = append(actions, "Failed to clear system proxy settings: "+err.Error())
	}

	if failures == 0 {
		return RepairResult{
			Success: true,
			Message: "Network repair operations completed successfully.",
			Actions: actions,
		}
	}
	if failures == attempted {
		return RepairResult{
			Success: false,
			Message: fmt.Sprintf("All %d network repair operations failed. Administrator privileges are usually required.", attempted),
			Actions: actions,
		}
	}
	return RepairResult{
		Success: false,
		Message: fmt.Sprintf("%d of %d network repair operations failed.", failures, attempted),
		Actions: actions,
	}
}

// Helper to check port reachability
func checkPort(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

// defaultGateway returns the IPv4 default gateway for the current OS, or "" when
// none can be determined. The old implementation hardcoded 192.168.1.1, which is
// simply wrong on most networks (192.168.0.1, 10.x, 172.16-31.x are all common).
func defaultGateway() string {
	switch runtime.GOOS {
	case "windows":
		out, err := hiddenCmd("route", "print", "-4").CombinedOutput()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			// "0.0.0.0          0.0.0.0      192.168.1.1     192.168.1.50     25"
			if len(f) >= 3 && f[0] == "0.0.0.0" && f[1] == "0.0.0.0" {
				if ip := net.ParseIP(f[2]); ip != nil && ip.To4() != nil {
					return f[2]
				}
			}
		}
	case "darwin":
		out, err := exec.Command("netstat", "-rn", "-f", "inet").Output()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			// "default            192.168.1.1        UGScg                 en0"
			if len(f) >= 2 && f[0] == "default" {
				if ip := net.ParseIP(f[1]); ip != nil && ip.To4() != nil {
					return f[1]
				}
			}
		}
	default: // linux and friends
		out, err := exec.Command("ip", "route", "show", "default").Output()
		if err == nil {
			// "default via 192.168.1.1 dev wlan0 proto dhcp metric 600"
			f := strings.Fields(string(out))
			for i, tok := range f {
				if tok == "via" && i+1 < len(f) {
					if ip := net.ParseIP(f[i+1]); ip != nil && ip.To4() != nil {
						return f[i+1]
					}
				}
			}
		}
		// Fallback for minimal images without iproute2.
		out, err = exec.Command("route", "-n").Output()
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 && f[0] == "0.0.0.0" {
				if ip := net.ParseIP(f[1]); ip != nil && ip.To4() != nil {
					return f[1]
				}
			}
		}
	}
	return ""
}

// findWintunDLL searches the same locations the engine resolves at runtime
// (core.EnsureWintunDriver) and returns the first plausible wintun.dll along with
// its size in bytes. An empty path means the driver is genuinely unavailable.
func findWintunDLL() (string, int64) {
	exe, err := os.Executable()
	if err != nil {
		return "", 0
	}
	execFolder := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(execFolder, "bin", "wintun.dll"),
		filepath.Join(execFolder, "wintun.dll"),
		"bin/wintun.dll",
		"wintun.dll",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 10000 {
			return c, info.Size()
		}
	}
	return "", 0
}
