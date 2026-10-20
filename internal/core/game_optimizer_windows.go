//go:build windows

package core

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// ApplyGamingOptimizations applies ultra-low latency TCP/IP registry tweaks for Windows
func (g *GamingOptimizer) ApplyGamingOptimizations() (*GamingOptimizationStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	status := &GamingOptimizationStatus{
		Active:              false,
		Supported:           true,
		OptimizedInterfaces: []string{},
		Message:             "Applying Windows gaming TCP/IP optimizations...",
	}

	// 1. Multimedia Network Throttling Optimization
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile
	sysProfileKey := `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, sysProfileKey, registry.SET_VALUE); err == nil {
		_ = k.SetDWordValue("NetworkThrottlingIndex", 0xffffffff) // Disable network throttling
		_ = k.SetDWordValue("SystemResponsiveness", 0x00000000)   // 100% CPU priority for gaming/networking
		k.Close()
		status.NetworkThrottlingTuned = true
		status.SystemResponsiveness = true
	} else {
		return status, fmt.Errorf("administrator privileges required: unable to access system profile registry (%w)", err)
	}

	// 2. Iterate TCP/IP Interface subkeys to enable TcpAckFrequency=1 and TCPNoDelay=1
	// HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces
	interfacesKey := `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
	ik, err := registry.OpenKey(registry.LOCAL_MACHINE, interfacesKey, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err == nil {
		subkeys, enumErr := ik.ReadSubKeyNames(-1)
		ik.Close()

		if enumErr == nil {
			for _, sub := range subkeys {
				subPath := interfacesKey + `\` + sub
				if sk, err := registry.OpenKey(registry.LOCAL_MACHINE, subPath, registry.SET_VALUE|registry.QUERY_VALUE); err == nil {
					_ = sk.SetDWordValue("TcpAckFrequency", 1) // Send ACK immediately for every packet
					_ = sk.SetDWordValue("TCPNoDelay", 1)      // Disable Nagle's algorithm
					_ = sk.SetDWordValue("TcpDelAckTicks", 0)  // Remove delayed ACK timer
					sk.Close()
					status.OptimizedInterfaces = append(status.OptimizedInterfaces, sub)
				}
			}
			if len(status.OptimizedInterfaces) > 0 {
				status.TcpAckFrequencyApplied = true
				status.TCPNoDelayApplied = true
			}
		}
	} else {
		return status, fmt.Errorf("administrator privileges required: unable to access network interfaces registry (%w)", err)
	}

	if !status.NetworkThrottlingTuned && len(status.OptimizedInterfaces) == 0 {
		return status, fmt.Errorf("administrator privileges required: failed to apply TCP/IP gaming tweaks (run as Administrator)")
	}

	g.active = true
	status.Active = true
	status.Supported = true
	status.Message = fmt.Sprintf("Ultra-low jitter gaming mode applied across %d network interfaces.", len(status.OptimizedInterfaces))
	return status, nil
}

// RevertGamingOptimizations restores Windows standard TCP/IP networking behavior
func (g *GamingOptimizer) RevertGamingOptimizations() (*GamingOptimizationStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	status := &GamingOptimizationStatus{
		Active:    false,
		Supported: true,
		Message:   "Restored to Windows default network stack settings.",
	}

	// 1. Revert System Profile
	sysProfileKey := `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, sysProfileKey, registry.SET_VALUE); err == nil {
		_ = k.SetDWordValue("NetworkThrottlingIndex", 10) // Windows Default = 10
		_ = k.SetDWordValue("SystemResponsiveness", 20)   // Windows Default = 20
		k.Close()
	}

	// 2. Revert Interfaces TCP settings
	interfacesKey := `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
	ik, err := registry.OpenKey(registry.LOCAL_MACHINE, interfacesKey, registry.ENUMERATE_SUB_KEYS)
	if err == nil {
		subkeys, enumErr := ik.ReadSubKeyNames(-1)
		ik.Close()

		if enumErr == nil {
			for _, sub := range subkeys {
				subPath := interfacesKey + `\` + sub
				if sk, err := registry.OpenKey(registry.LOCAL_MACHINE, subPath, registry.SET_VALUE); err == nil {
					_ = sk.DeleteValue("TcpAckFrequency")
					_ = sk.DeleteValue("TCPNoDelay")
					_ = sk.DeleteValue("TcpDelAckTicks")
					sk.Close()
				}
			}
		}
	}

	g.active = false
	return status, nil
}
