//go:build windows

package core

import (
	"golang.org/x/sys/windows/registry"
)

// readGamingOptimizationState inspects the real Windows registry values written
// by ApplyGamingOptimizations, so the reported status reflects the machine's
// actual configuration instead of an in-memory flag that resets on restart.
func readGamingOptimizationState() GamingOptimizationStatus {
	status := GamingOptimizationStatus{
		Supported:           true,
		OptimizedInterfaces: []string{},
	}

	// 1. Multimedia SystemProfile throttling values
	const sysProfileKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, sysProfileKey, registry.QUERY_VALUE); err == nil {
		if v, _, vErr := k.GetIntegerValue("NetworkThrottlingIndex"); vErr == nil && v == 0xffffffff {
			status.NetworkThrottlingTuned = true
		}
		if v, _, vErr := k.GetIntegerValue("SystemResponsiveness"); vErr == nil && v == 0 {
			status.SystemResponsiveness = true
		}
		_ = k.Close()
	}

	// 2. Per-interface TCP tweaks
	const interfacesKey = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
	ackApplied := false
	noDelayApplied := false
	if ik, err := registry.OpenKey(registry.LOCAL_MACHINE, interfacesKey, registry.ENUMERATE_SUB_KEYS); err == nil {
		subkeys, enumErr := ik.ReadSubKeyNames(-1)
		_ = ik.Close()
		if enumErr == nil {
			for _, sub := range subkeys {
				subPath := interfacesKey + `\` + sub
				sk, openErr := registry.OpenKey(registry.LOCAL_MACHINE, subPath, registry.QUERY_VALUE)
				if openErr != nil {
					continue
				}
				ack := false
				if v, _, vErr := sk.GetIntegerValue("TcpAckFrequency"); vErr == nil && v == 1 {
					ack = true
				}
				noDelay := false
				if v, _, vErr := sk.GetIntegerValue("TCPNoDelay"); vErr == nil && v == 1 {
					noDelay = true
				}
				_ = sk.Close()

				if ack || noDelay {
					status.OptimizedInterfaces = append(status.OptimizedInterfaces, sub)
				}
				if ack {
					ackApplied = true
				}
				if noDelay {
					noDelayApplied = true
				}
			}
		}
	}

	status.TcpAckFrequencyApplied = ackApplied
	status.TCPNoDelayApplied = noDelayApplied
	return status
}
