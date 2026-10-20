//go:build windows

package sysproc

import (
	"context"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	procNotifyIpInterfaceChange = iphlpapi.NewProc("NotifyIpInterfaceChange")
	procCancelMibChangeNotify2  = iphlpapi.NewProc("CancelMibChangeNotify2")
)

// AF_UNSPEC asks NotifyIpInterfaceChange to report both IPv4 and IPv6
// interfaces. Windows does not export this for the iphlpapi entry point, so it
// is spelled out here.
const addressFamilyUnspec = 0

// MIB_NOTIFICATION_TYPE values delivered to the callback. Only the two
// add/delete cases matter; MibInitialNotification cannot occur because the
// registration passes InitialNotification = FALSE.
const (
	mibAddInstance    = 0
	mibDeleteInstance = 1
)

// windowsNotificationHandle is the registration handle returned by
// NotifyIpInterfaceChange and consumed by CancelMibChangeNotify2.
type windowsNotificationHandle uintptr

// ipInterfaceChangeCallback is the signature Windows calls on every interface
// change:
//
//	VOID NETIOAPI_API_ Callback(PVOID, PMIB_IPINTERFACE_ROW, MIB_NOTIFICATION_TYPE)
//
// The row is deliberately ignored. It describes a single interface, while the
// only question the caller needs answered is "did the set change at all", and
// the answer is derived from a fresh fingerprint instead of from this argument.
func ipInterfaceChangeCallback(_, _, notificationType uintptr) uintptr {
	if notificationType == mibAddInstance || notificationType == mibDeleteInstance {
		if fn := loadInterfaceChangeSink(); fn != nil {
			fn()
		}
	}
	return 0
}

var (
	// The callback pointer must outlive the registration: Windows keeps calling
	// it until CancelMibChangeNotify2, and the Go runtime does not scan the
	// value once it has been handed to the OS. Holding it in a package-level
	// variable is what keeps it reachable.
	interfaceChangeCallbackOnce sync.Once
	interfaceChangeCallbackPtr  uintptr

	interfaceChangeSinkMu sync.RWMutex
	interfaceChangeSink   func()
)

func loadInterfaceChangeSink() func() {
	interfaceChangeSinkMu.RLock()
	defer interfaceChangeSinkMu.RUnlock()
	return interfaceChangeSink
}

// registerInterfaceNotifications subscribes to iphlpapi interface-change
// notifications and calls onChange for each one until ctx is cancelled.
//
// Failure is not fatal and is never surfaced: the caller in WatchNetworkChanges
// always has the fingerprint poller as a fallback, so a machine where this
// registration fails still reacts to network changes, just less promptly.
func registerInterfaceNotifications(ctx context.Context, onChange func()) {
	interfaceChangeCallbackOnce.Do(func() {
		interfaceChangeCallbackPtr = windows.NewCallback(ipInterfaceChangeCallback)
	})

	interfaceChangeSinkMu.Lock()
	interfaceChangeSink = onChange
	interfaceChangeSinkMu.Unlock()

	var handle windowsNotificationHandle
	ret, _, _ := procNotifyIpInterfaceChange.Call(
		uintptr(addressFamilyUnspec),
		interfaceChangeCallbackPtr,
		0, // CallerContext
		0, // InitialNotification = FALSE
		uintptr(unsafe.Pointer(&handle)),
	)
	if ret != 0 || handle == 0 {
		interfaceChangeSinkMu.Lock()
		interfaceChangeSink = nil
		interfaceChangeSinkMu.Unlock()
		return
	}

	go func() {
		<-ctx.Done()
		// Cancel first, then drop the sink. Cancelling guarantees Windows stops
		// invoking the callback, so clearing the sink afterwards cannot race
		// with an in-flight call that would find it nil.
		_, _, _ = procCancelMibChangeNotify2.Call(uintptr(handle))
		interfaceChangeSinkMu.Lock()
		interfaceChangeSink = nil
		interfaceChangeSinkMu.Unlock()
	}()
}
