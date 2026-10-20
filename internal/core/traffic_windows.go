//go:build windows

package core

import (
	"syscall"
	"unsafe"
)

var (
	iphlpapi         = syscall.NewLazyDLL("iphlpapi.dll")
	getIfTable2Proc  = iphlpapi.NewProc("GetIfTable2")
	freeMibTableProc = iphlpapi.NewProc("FreeMibTable")
)

type MIB_IF_ROW2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGuid               [16]byte
	Alias                       [257]uint16
	Description                 [257]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [32]byte
	PermanentPhysicalAddress    [32]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags uint32
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 [16]byte
	ConnectionType              uint32
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

type MIB_IF_TABLE2 struct {
	NumEntries uint32
	_          uint32 // padding
	Table      [1]MIB_IF_ROW2
}

// QueryTunnelAdapterBytes scans network interfaces to find the active VPN/TUN adapter (e.g. Wintun, TAP, NexVPN-TUN)
func QueryTunnelAdapterBytes() (rx int64, tx int64, found bool) {
	var pTable *MIB_IF_TABLE2
	ret, _, _ := getIfTable2Proc.Call(uintptr(unsafe.Pointer(&pTable)))
	if ret != 0 || pTable == nil {
		return 0, 0, false
	}
	defer func() { _, _, _ = freeMibTableProc.Call(uintptr(unsafe.Pointer(pTable))) }()

	count := int(pTable.NumEntries)
	if count <= 0 {
		return 0, 0, false
	}

	// Pointer arithmetic to iterate through rows
	firstRow := (*MIB_IF_ROW2)(unsafe.Pointer(&pTable.Table[0]))
	rowSize := unsafe.Sizeof(MIB_IF_ROW2{})

	for i := 0; i < count; i++ {
		row := (*MIB_IF_ROW2)(unsafe.Pointer(uintptr(unsafe.Pointer(firstRow)) + uintptr(i)*rowSize))

		desc := syscall.UTF16ToString(row.Description[:])
		alias := syscall.UTF16ToString(row.Alias[:])

		// Match Wintun, TAP, OpenVPN, Sing-box, or NexVPN tunnel interfaces
		if isTunnelInterface(desc, alias) {
			return int64(row.InOctets), int64(row.OutOctets), true
		}
	}

	return 0, 0, false
}

func isTunnelInterface(desc, alias string) bool {
	matches := []string{"wintun", "tap", "nexvpn", "sing-box", "wireguard", "openvpn", "tun"}
	descLower := toLowerStr(desc)
	aliasLower := toLowerStr(alias)

	for _, m := range matches {
		if containsStr(descLower, m) || containsStr(aliasLower, m) {
			return true
		}
	}
	return false
}

func toLowerStr(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
