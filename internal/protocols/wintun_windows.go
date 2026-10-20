//go:build windows

package protocols

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"nexvpn/internal/sysproc"
)

// networkClassRoot is the registry key under which Windows records, for every
// network class device, the adapter name it is bound to and the PnP instance id
// behind it. See wintunRegistryBoundNames.
const networkClassRoot = `SYSTEM\CurrentControlSet\Control\Network\{4d36e972-e325-11ce-bfc1-08002be10318}`

var (
	wintunGUID = GUID{
		Data1: 0x5a1e6e0d,
		Data2: 0x3cb3,
		Data3: 0x1a48,
		Data4: [8]byte{0x8f, 0x33, 0xe2, 0x25, 0x26, 0x82, 0xea, 0xb4},
	}
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// EnsureWintunAdapter ensures a dedicated, isolated Wintun adapter exists on Windows.
func EnsureWintunAdapter(binDir string, adapterName string) error {
	dllPath := filepath.Join(binDir, "wintun.dll")
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return fmt.Errorf("failed to load wintun.dll: %w", err)
	}
	defer dll.Release()

	procOpenAdapter, err := dll.FindProc("WintunOpenAdapter")
	if err != nil {
		return err
	}
	procCreateAdapter, err := dll.FindProc("WintunCreateAdapter")
	if err != nil {
		return err
	}
	procCloseAdapter, err := dll.FindProc("WintunCloseAdapter")
	if err != nil {
		return err
	}

	namePtr, err := syscall.UTF16PtrFromString(adapterName)
	if err != nil {
		return err
	}
	tunnelTypePtr, err := syscall.UTF16PtrFromString("OpenVPN")
	if err != nil {
		return err
	}

	// Try opening existing adapter
	handle, _, _ := procOpenAdapter.Call(uintptr(unsafe.Pointer(namePtr)))
	if handle != 0 {
		procCloseAdapter.Call(handle)
		return nil
	}

	// Adapter doesn't exist yet, attempt to create it
	handle, _, createErr := procCreateAdapter.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(tunnelTypePtr)),
		uintptr(unsafe.Pointer(&wintunGUID)),
	)
	if handle == 0 {
		return fmt.Errorf("failed to create wintun adapter %s: %v", adapterName, createErr)
	}

	procCloseAdapter.Call(handle)
	return nil
}

// WintunDLLPresent reports whether wintun.dll can be found next to the core.
//
// When it cannot, a TUN inbound can never work, so the readiness gate in
// Start() must not sit on its full window waiting for an adapter that will
// never appear — the core fails fast with its own, clearer error instead.
func WintunDLLPresent(binDir string) bool {
	if binDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(binDir, "wintun.dll"))
	return err == nil
}

// WintunAdapterReady reports whether a Wintun adapter with this exact name
// currently exists.
//
// This is the readiness signal for a TUN inbound. `WintunOpenAdapter` succeeds
// the moment the adapter has been created — which is precisely the step that
// fails, and takes the whole core down with it, when a ghost adapter is left
// over from a previous crash. Polling this lets Start() tell "the TUN adapter
// is up" apart from "the process is alive but the TUN inbound never came up".
//
// Opening an adapter that sing-box already holds is safe: Wintun allows
// concurrent opens, and the handle is closed immediately.
func WintunAdapterReady(binDir, adapterName string) bool {
	if binDir == "" || adapterName == "" {
		return false
	}
	dll, err := syscall.LoadDLL(filepath.Join(binDir, "wintun.dll"))
	if err != nil {
		return false
	}
	defer dll.Release()

	procOpenAdapter, err := dll.FindProc("WintunOpenAdapter")
	if err != nil {
		return false
	}
	procCloseAdapter, err := dll.FindProc("WintunCloseAdapter")
	if err != nil {
		return false
	}

	namePtr, err := syscall.UTF16PtrFromString(adapterName)
	if err != nil {
		return false
	}

	handle, _, _ := procOpenAdapter.Call(uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return false
	}
	procCloseAdapter.Call(handle)
	return true
}

// WindowsVisibleAdapterNames returns the names of the network adapters Windows
// itself can enumerate, lower-cased.
//
// Returns nil when the list cannot be determined. Callers MUST treat nil as
// "assume nothing is ghosted", never as "everything is ghosted" — a probe that
// did not run must not be able to rename every adapter.
//
// Read through GetAdaptersAddresses rather than `Get-NetAdapter`. The cmdlet
// costs a full PowerShell launch — ~5 s on the reference machine, where the
// NetAdapter and PnpDevice modules have to autoload — and this probe sits on the
// critical path of every connection attempt, where it is the single largest
// contributor to the "pressing Connect does nothing for twenty seconds" report.
// The Win32 call answers the same question in microseconds.
func WindowsVisibleAdapterNames() map[string]bool {
	// Only the name is wanted, so every address class is skipped; the adapter
	// entries themselves are still returned. INCLUDE_ALL_INTERFACES is what
	// makes an adapter with no address at all visible, which is the state a
	// Wintun adapter spends most of its life in.
	flags := uint32(windows.GAA_FLAG_INCLUDE_ALL_INTERFACES |
		windows.GAA_FLAG_SKIP_UNICAST |
		windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST |
		windows.GAA_FLAG_SKIP_DNS_SERVER)

	var size uint32
	err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, nil, &size)
	// The first call is expected to fail with ERROR_BUFFER_OVERFLOW: that is how
	// the required size is reported.
	if err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return nil
	}
	if size == 0 {
		return nil
	}

	buf := make([]byte, size)
	addrs := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, addrs, &size); err != nil {
		return nil
	}

	names := make(map[string]bool)
	for a := addrs; a != nil; a = a.Next {
		if a.FriendlyName == nil {
			continue
		}
		if name := strings.ToLower(strings.TrimSpace(windows.UTF16PtrToString(a.FriendlyName))); name != "" {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// wintunRegistryBoundNames returns the lower-cased adapter names that Windows
// binds to a Wintun software device, read from the network class registry key.
//
// This is the half of the ghost test that the Wintun DLL cannot supply. Measured
// on a machine with a ghost, `WintunOpenAdapter("NexVPN-TUN")` returned NULL —
// the driver does *not* answer for the name — while the registry still carried
//
//	{4d36e972-...}\{A5EA6E0D-3CB3-1A48-8F33-E2252682EAB4}\Connection
//	    Name          = NexVPN-TUN
//	    PnPInstanceId = SWD\Wintun\{A5EA6E0D-3CB3-1A48-8F33-E2252682EAB4}
//
// and `openvpn --show-adapters` listed `'NexVPN-TUN' {...} wintun` even though
// `Get-NetAdapter` and `Get-PnpDevice -Class Net` could not see it. So the
// binding is the durable evidence, and the DLL call is not.
//
// Returns nil when the key cannot be read or binds no Wintun name at all. The
// second case matters: on a clean machine nothing is ghosted, and reporting nil
// keeps that from being mistaken for "every name is taken".
func wintunRegistryBoundNames() map[string]bool {
	// Read through the registry API rather than shelling out to PowerShell, for
	// the same latency reason as WindowsVisibleAdapterNames: this is the second
	// half of the ghost probe, and it runs on the critical path of every connect.
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, networkClassRoot, registry.READ)
	if err != nil {
		return nil
	}
	defer key.Close()

	subKeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	names := make(map[string]bool)
	for _, sub := range subKeys {
		conn, err := registry.OpenKey(key, sub+`\Connection`, registry.READ)
		if err != nil {
			continue
		}
		name, _, nameErr := conn.GetStringValue("Name")
		pnp, _, pnpErr := conn.GetStringValue("PnPInstanceId")
		conn.Close()

		if nameErr != nil || pnpErr != nil || name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(pnp), `swd\wintun\`) {
			names[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// WintunGhostScan is one pass over the machine's adapter state, enough to decide
// whether any given name is a ghost.
//
// Both halves are collected together because each costs a PowerShell launch, and
// a caller that checks several candidate names (chooseTunInterfaceName checks up
// to ten) must not pay for that per name.
type WintunGhostScan struct {
	// Visible is the set of names Windows can enumerate, or nil if unknown.
	Visible map[string]bool
	// Bound is the set of names the registry binds to a Wintun device, or nil if
	// unknown.
	Bound map[string]bool
}

// ScanWintunGhosts collects the evidence needed to spot a ghost adapter.
func ScanWintunGhosts() WintunGhostScan {
	return WintunGhostScan{
		Visible: WindowsVisibleAdapterNames(),
		Bound:   wintunRegistryBoundNames(),
	}
}

// Ghosted reports whether name is a ghost: the registry binds it to a Wintun
// device, but Windows has no such adapter.
//
// A ghost is what makes a TUN core look healthy and then die. The core is handed
// the name, `WintunCreateAdapter` refuses it as taken, the adapter never comes
// up, and the readiness probe — which only asks whether *an* adapter with that
// name exists — reports success the moment the process starts.
//
// Returns false whenever either half of the evidence is unknown. A probe that
// did not run must never be able to rename adapters.
func (s WintunGhostScan) Ghosted(name string) bool {
	if s.Visible == nil || s.Bound == nil {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return false
	}
	// A real adapter is exactly what the core wants to reuse.
	if s.Visible[key] {
		return false
	}
	return s.Bound[key]
}

// NexVPNWintunAdapterNames lists every adapter name this application is allowed
// to create. It is the single source of truth for both the presence check and
// the removal pass, so the two can never disagree about what "ours" means.
//
// The names match the candidates the engine offers a TUN core plus the dedicated
// OpenVPN adapter name. Nothing else may appear here: a third-party Wintun
// adapter (WireGuard, another VPN client) belongs to its own product and must
// never be deleted by this application.
func NexVPNWintunAdapterNames() []string {
	return []string{
		"NexVPN-TUN",
		"NexVPN-TUN-2",
		"NexVPN-TUN-3",
		"NexVPN-TUN-4",
		"NexVPN-TUN-5",
		"NexVPN-TUN-6",
		"NexVPN-TUN-7",
		"NexVPN-TUN-8",
		"NexVPN-TUN-9",
		"NexVPN-OVPN",
	}
}

// NexVPNWintunAdapterPresent reports whether a scan saw any adapter name
// belonging to NexVPN, either as a live adapter or as a registry binding.
//
// A binding alone counts: that is the ghost case, and a ghost is the one that
// actually blocks a core. An unknown scan (nil maps) reports false, so a probe
// that did not run can never trigger a removal.
func NexVPNWintunAdapterPresent(scan WintunGhostScan) bool {
	for _, name := range NexVPNWintunAdapterNames() {
		key := strings.ToLower(strings.TrimSpace(name))
		if scan.Visible[key] || scan.Bound[key] {
			return true
		}
	}
	return false
}

// NexVPNWintunAdapterLive reports whether a scan saw one of NexVPN's names as an
// adapter Windows can actually enumerate — a device with a driver instance, not
// just a registry binding left behind.
//
// This is the question a *new* Wintun allocation has to be asked, and it is not
// the same question NexVPNWintunAdapterPresent answers. OpenVPN's "All Wintun
// network adapters on this system are currently in use or disabled" is about
// live adapters holding the shared pool; a ghost holds nothing, and its remedy
// is a different name (chooseTunInterfaceName), not a wait. Counting ghosts as
// blockers made the 2026-10-12 log poll for the full budget on every OpenVPN
// connect and still fall back to tap-windows6.
//
// An unknown scan (nil Visible) reports false, so a probe that did not run can
// never block a driver choice.
func NexVPNWintunAdapterLive(scan WintunGhostScan) bool {
	for _, name := range NexVPNWintunAdapterNames() {
		if scan.Visible[strings.ToLower(strings.TrimSpace(name))] {
			return true
		}
	}
	return false
}

// ClearStaleNexVPNWintunAdapters deletes NexVPN's own Wintun adapters outright.
//
// Re-enabling an adapter through `netsh ... admin=enable` cannot repair the
// condition that actually blocks a reconnect: a *ghost*, whose name is still
// bound in the network class registry to a Wintun software device while Windows
// has no adapter under it. The core then dies with
//
//	configure tun interface: (create adapter: Cannot create a file when that
//	file already exists. | open existing adapter: Element not found.)
//
// and no amount of netsh toggling helps, because the interface is not in the
// adapter list for netsh to find in the first place. So this does three passes:
//
//  1. `Remove-NetAdapter` for a live-but-unwanted adapter.
//  2. `Remove-PnpDevice` by the PnP instance id recorded in the registry binding
//     — the only handle a ghost has, and the reason a ghost is now removable at
//     all rather than merely avoidable.
//  3. `Remove-PnpDevice` by friendly name, for the case where the device is
//     present but not enumerable as a network adapter.
//
// Scoped strictly by exact name to NexVPN's own adapters.
func ClearStaleNexVPNWintunAdapters() {
	script := `$names = @('NexVPN-TUN','NexVPN-OVPN','NexVPN-TUN-2','NexVPN-TUN-3','NexVPN-TUN-4','NexVPN-TUN-5','NexVPN-TUN-6','NexVPN-TUN-7','NexVPN-TUN-8','NexVPN-TUN-9')
$root = 'HKLM:\SYSTEM\CurrentControlSet\Control\Network\{4d36e972-e325-11ce-bfc1-08002be10318}'
$bindings = @()
Get-ChildItem $root -ErrorAction SilentlyContinue | ForEach-Object {
  $c = Join-Path $_.PSPath 'Connection'
  if (Test-Path $c) {
    $p = Get-ItemProperty $c -ErrorAction SilentlyContinue
    if ($p.Name -and $p.PnPInstanceId) {
      $bindings += [pscustomobject]@{ Name = $p.Name; Pnp = $p.PnPInstanceId }
    }
  }
}
foreach ($n in $names) {
  Get-NetAdapter -Name $n -IncludeHidden -ErrorAction SilentlyContinue |
    Remove-NetAdapter -Confirm:$false -ErrorAction SilentlyContinue
  foreach ($b in ($bindings | Where-Object { $_.Name -eq $n })) {
    Remove-PnpDevice -InstanceId $b.Pnp -Confirm:$false -ErrorAction SilentlyContinue
  }
  Get-PnpDevice -Class Net -ErrorAction SilentlyContinue |
    Where-Object { $_.FriendlyName -eq $n } |
    Remove-PnpDevice -Confirm:$false -ErrorAction SilentlyContinue
}`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	_ = cmd.Run()
}
