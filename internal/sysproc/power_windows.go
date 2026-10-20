//go:build windows

package sysproc

import (
	"context"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	powrprof = windows.NewLazySystemDLL("powrprof.dll")

	procPowerRegisterSuspendResumeNotification   = powrprof.NewProc("PowerRegisterSuspendResumeNotification")
	procPowerUnregisterSuspendResumeNotification = powrprof.NewProc("PowerUnregisterSuspendResumeNotification")

	procGetSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")
)

// DEVICE_NOTIFY_CALLBACK tells PowerRegisterSuspendResumeNotification that the
// recipient is a DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS structure holding a
// function pointer, rather than a window handle. Using the callback form avoids
// having to create a message-only window and run a message pump just to hear
// about suspend/resume.
const deviceNotifyCallback = 2

// PBT_* power-broadcast codes delivered to the callback.
const (
	pbtApmsuspend         = 0x0004
	pbtApmresumecritical  = 0x0006
	pbtApmresumesuspend   = 0x0007
	pbtApmresumeautomatic = 0x0012
	pbtPowerSettingChange = 0x8013
	acLineStatusOffline   = 0
	acLineStatusOnline    = 1
	acLineStatusUnknown   = 255
	powerPollInterval     = 20 * time.Second
)

// deviceNotifySubscribeParameters mirrors
// DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS from powrprof.h.
type deviceNotifySubscribeParameters struct {
	Callback uintptr
	Context  uintptr
}

// systemPowerStatus mirrors SYSTEM_POWER_STATUS from winbase.h.
type systemPowerStatus struct {
	ACLineStatus        uint8
	BatteryFlag         uint8
	BatteryLifePercent  uint8
	SystemStatusFlag    uint8
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var (
	// Both of these must stay reachable for as long as the registration lives:
	// the OS holds a raw pointer to the parameters struct and a raw function
	// pointer, neither of which the Go garbage collector can see.
	powerSubscribeParams deviceNotifySubscribeParameters
	powerCallbackPtr     uintptr

	powerSinkMu sync.RWMutex
	powerSink   func(string)
)

func loadPowerSink() func(string) {
	powerSinkMu.RLock()
	defer powerSinkMu.RUnlock()
	return powerSink
}

// powerNotifyCallback is invoked on a Windows-owned thread. It only translates
// the notification code and hands it off; anything more would run arbitrary Go
// code on a thread the power manager may be waiting on.
func powerNotifyCallback(_, notifyType, _ uintptr) uintptr {
	var kind string
	switch notifyType {
	case pbtApmsuspend:
		kind = PowerSuspend
	case pbtApmresumeautomatic, pbtApmresumesuspend, pbtApmresumecritical:
		kind = PowerResume
	default:
		// PBT_POWERSETTINGCHANGE and anything else: not a transition we act on.
		return 0
	}
	if fn := loadPowerSink(); fn != nil {
		fn(kind)
	}
	return 0
}

// currentACLineStatus reads the mains/battery state, returning
// acLineStatusUnknown when the value cannot be read (desktops without a battery
// report 255).
func currentACLineStatus() uint8 {
	var status systemPowerStatus
	ret, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 {
		return acLineStatusUnknown
	}
	return status.ACLineStatus
}

// registerPowerNotifications subscribes to suspend/resume notifications and
// polls the mains state, calling onChange until ctx is cancelled.
//
// Failure of the registration is silent by design: the caller treats this as an
// optimisation over the existing health monitor, never as the mechanism that
// keeps the tunnel alive.
func registerPowerNotifications(ctx context.Context, onChange func(string)) {
	powerSinkMu.Lock()
	powerSink = onChange
	powerSinkMu.Unlock()

	// The AC poller is independent of the registration below, so it is started
	// first and is unaffected by a registration failure.
	go func() {
		ticker := time.NewTicker(powerPollInterval)
		defer ticker.Stop()

		last := currentACLineStatus()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := currentACLineStatus()
				if now == last || now == acLineStatusUnknown || last == acLineStatusUnknown {
					// An unknown reading is not a transition; treating it as one
					// would fire a spurious "unplugged" event on every desktop.
					if now != acLineStatusUnknown {
						last = now
					}
					continue
				}
				last = now
				if now == acLineStatusOnline {
					onChange(PowerAC)
				} else {
					onChange(PowerBattery)
				}
			}
		}
	}()

	powerCallbackPtr = windows.NewCallback(powerNotifyCallback)
	powerSubscribeParams = deviceNotifySubscribeParameters{
		Callback: powerCallbackPtr,
		Context:  0,
	}

	var handle uintptr
	ret, _, _ := procPowerRegisterSuspendResumeNotification.Call(
		uintptr(deviceNotifyCallback),
		uintptr(unsafe.Pointer(&powerSubscribeParams)),
		uintptr(unsafe.Pointer(&handle)),
	)
	if ret != 0 || handle == 0 {
		return
	}

	go func() {
		<-ctx.Done()
		// Unregister before dropping the sink: the OS must stop calling the
		// callback before the sink becomes nil, or an in-flight notification
		// would find nothing to call.
		_, _, _ = procPowerUnregisterSuspendResumeNotification.Call(handle)
		powerSinkMu.Lock()
		powerSink = nil
		powerSinkMu.Unlock()
	}()
}
