//go:build windows

package tray

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

//go:embed icon.ico
var embeddedIco []byte

var (
	shell32                  = syscall.NewLazyDLL("shell32.dll")
	user32                   = syscall.NewLazyDLL("user32.dll")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	shellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")
	extractIconExW           = shell32.NewProc("ExtractIconExW")
	getModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	loadIconW                = user32.NewProc("LoadIconW")
	findWindowW              = user32.NewProc("FindWindowW")
	showWindow               = user32.NewProc("ShowWindow")
	setForegroundWin         = user32.NewProc("SetForegroundWindow")
	createPopupMenu          = user32.NewProc("CreatePopupMenu")
	appendMenuW              = user32.NewProc("AppendMenuW")
	trackPopupMenuEx         = user32.NewProc("TrackPopupMenuEx")
	destroyMenu              = user32.NewProc("DestroyMenu")
	getCursorPos             = user32.NewProc("GetCursorPos")
	defWindowProcW           = user32.NewProc("DefWindowProcW")
	registerClassExW         = user32.NewProc("RegisterClassExW")
	createWindowExW          = user32.NewProc("CreateWindowExW")
	destroyWindow            = user32.NewProc("DestroyWindow")
	getMessageW              = user32.NewProc("GetMessageW")
	translateMessage         = user32.NewProc("TranslateMessage")
	dispatchMessageW         = user32.NewProc("DispatchMessageW")
	sendMessageW             = user32.NewProc("SendMessageW")
	setClassLongPtrW         = user32.NewProc("SetClassLongPtrW")
	createIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
)

const (
	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	WM_USER         = 0x0400
	WM_TRAYCALLBACK = WM_USER + 101

	WM_SETICON = 0x0080
	ICON_SMALL = 0
	ICON_BIG   = 1

	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	MF_STRING       = 0x00000000
	MF_GRAYED       = 0x00000001
	MF_DISABLED     = 0x00000002
	MF_SEPARATOR    = 0x00000800
	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTBUTTON = 0x0002

	SW_RESTORE = 9
	SW_SHOW    = 5

	IDI_APPLICATION = 32512

	CMD_STATUS          = 1000
	CMD_SHOW            = 1001
	CMD_TOGGLE_CONNECT  = 1002
	CMD_CONNECT_FASTEST = 1003
	CMD_PANIC_WIPE      = 1004
	CMD_EXIT            = 1005

	// Diagnostics commands. Numbered with the fixed block, not from
	// CMD_SERVER_BASE, because the server range is dynamic.
	CMD_OPEN_REPORT = 1006
	CMD_TOGGLE_GPU  = 1007

	// CMD_SERVER_BASE is the first command id reserved for the dynamic
	// "Servers" submenu. Entries are numbered from here upwards, so the handler
	// can tell a server selection apart from a fixed command without a lookup
	// table that would have to stay in sync with the menu.
	CMD_SERVER_BASE = 2000

	MF_POPUP = 0x00000010
)

type POINT struct {
	X int32
	Y int32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	TimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
}

var (
	trayMu            sync.RWMutex
	activeNID         *NOTIFYICONDATAW
	trayHWnd          uintptr
	isConnected       bool
	profileName       string
	latencyMs         int
	currentLanguage   string = "fa"
	toggleConnectFunc func()
	fastConnectFunc   func()
	panicWipeFunc     func()
	showFunc          func()
	exitFunc          func()
	isShowingAtomic   uint32

	// serverListFunc supplies the entries for the "Servers" submenu, and
	// serverSelectFunc receives the id of the one the user picked. Both are
	// injected by the app so the tray package stays free of profile knowledge.
	serverListFunc   func() []ServerItem
	serverSelectFunc func(string)

	// Diagnostics entries. The tray is the only surface reachable when the window
	// cannot be drawn — a black WebView2 pane leaves the UI unusable but the icon
	// alive — which is why the launch report and the rendering lever live here
	// rather than only inside the app.
	//
	// gpuStateFunc reports whether GPU rendering is currently off, so the menu
	// item can read as the action it performs ("turn off" vs "turn back on")
	// instead of a toggle whose state the user has to guess.
	openReportFunc func()
	gpuToggleFunc  func()
	gpuStateFunc   func() bool
)

// SetDiagnosticsHandlers registers the tray's diagnostics entries. Any of them
// may be nil, and a nil handler simply hides its item, so a build without the
// report tool loses the menu row instead of showing a dead click target.
func SetDiagnosticsHandlers(onOpenReport, onToggleGPU func(), gpuDisabled func() bool) {
	trayMu.Lock()
	openReportFunc = onOpenReport
	gpuToggleFunc = onToggleGPU
	gpuStateFunc = gpuDisabled
	trayMu.Unlock()
}

// SetServerListProvider registers the callback that supplies the tray's server
// submenu. Returning an empty slice hides the submenu entirely, which is the
// right behaviour when the user has no profiles yet.
func SetServerListProvider(fn func() []ServerItem) {
	trayMu.Lock()
	serverListFunc = fn
	trayMu.Unlock()
}

// SetServerSelectHandler registers the callback invoked with the id of the
// profile the user picked from the tray submenu.
func SetServerSelectHandler(fn func(string)) {
	trayMu.Lock()
	serverSelectFunc = fn
	trayMu.Unlock()
}

// SetToggleConnectHandler registers a callback for toggling connection (connect / disconnect)
func SetToggleConnectHandler(fn func()) {
	trayMu.Lock()
	toggleConnectFunc = fn
	trayMu.Unlock()
}

// SetFastConnectHandler registers a callback for connecting to the fastest profile
func SetFastConnectHandler(fn func()) {
	trayMu.Lock()
	fastConnectFunc = fn
	trayMu.Unlock()
}

// SetPanicWipeHandler registers a callback for panic disconnecting and wiping proxy
func SetPanicWipeHandler(fn func()) {
	trayMu.Lock()
	panicWipeFunc = fn
	trayMu.Unlock()
}

// SetDisconnectHandler backward-compatibility helper
func SetDisconnectHandler(fn func()) {
	SetToggleConnectHandler(fn)
}

// SetShowHandler registers a callback invoked when Show or left click is triggered
func SetShowHandler(fn func()) {
	trayMu.Lock()
	showFunc = fn
	trayMu.Unlock()
}

// SetExitHandler registers a callback invoked when Exit is clicked from tray menu
func SetExitHandler(fn func()) {
	trayMu.Lock()
	exitFunc = fn
	trayMu.Unlock()
}

// UpdateTrayState syncs the current connection state and updates tooltip
func UpdateTrayState(connected bool, profile string, latency int) {
	trayMu.Lock()
	isConnected = connected
	profileName = profile
	latencyMs = latency
	lang := currentLanguage
	trayMu.Unlock()

	tip := "NexVPN Pro - Disconnected"
	if lang == "fa" {
		tip = "نکس وی‌پی‌ان پرو - غیرفعال"
		if connected {
			if latency > 0 {
				tip = fmt.Sprintf("نکس وی‌پی‌ان: متصل به %s (%dms)", profile, latency)
			} else {
				tip = fmt.Sprintf("نکس وی‌پی‌ان: متصل به %s", profile)
			}
		}
	} else {
		if connected {
			if latency > 0 {
				tip = fmt.Sprintf("NexVPN: Connected to %s (%dms)", profile, latency)
			} else {
				tip = fmt.Sprintf("NexVPN: Connected to %s", profile)
			}
		}
	}

	UpdateTooltip(tip)
}

// SetTrayLanguage changes the active display language in tray menu
func SetTrayLanguage(lang string) {
	trayMu.Lock()
	if lang == "en" || lang == "fa" {
		currentLanguage = lang
	}
	conn := isConnected
	prof := profileName
	lat := latencyMs
	trayMu.Unlock()

	UpdateTrayState(conn, prof, lat)
}

func getBestHIcon(width, height int32) uintptr {
	// First attempt: Create directly from embedded icon bytes
	if len(embeddedIco) > 0 {
		hIcon, _, _ := createIconFromResourceEx.Call(
			uintptr(unsafe.Pointer(&embeddedIco[0])),
			uintptr(len(embeddedIco)),
			1,          // fIcon = TRUE
			0x00030000, // dwVersion
			uintptr(width),
			uintptr(height),
			0, // LR_DEFAULTCOLOR
		)
		if hIcon != 0 {
			return hIcon
		}
	}

	// Second attempt: Extract from running executable
	var hIconLarge, hIconSmall uintptr
	if exePathStr, err := os.Executable(); err == nil {
		if exePath, err := syscall.UTF16PtrFromString(exePathStr); err == nil {
			_, _, _ = extractIconExW.Call(
				uintptr(unsafe.Pointer(exePath)),
				0,
				uintptr(unsafe.Pointer(&hIconLarge)),
				uintptr(unsafe.Pointer(&hIconSmall)),
				1,
			)
		}
	}

	if width <= 32 && hIconSmall != 0 {
		return hIconSmall
	}
	if hIconLarge != 0 {
		return hIconLarge
	}
	if hIconSmall != 0 {
		return hIconSmall
	}

	hDefault, _, _ := loadIconW.Call(0, uintptr(IDI_APPLICATION))
	return hDefault
}

func trayWndProc(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == WM_TRAYCALLBACK {
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			ShowMainWindow()
		case WM_RBUTTONUP:
			showContextMenu(hWnd)
		}
		return 0
	}
	ret, _, _ := defWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret
}

func showContextMenu(hWnd uintptr) {
	hMenu, _, _ := createPopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer func() { _, _, _ = destroyMenu.Call(hMenu) }()

	trayMu.RLock()
	conn := isConnected
	pName := profileName
	lat := latencyMs
	lang := currentLanguage
	openReport := openReportFunc
	gpuToggle := gpuToggleFunc
	gpuState := gpuStateFunc
	trayMu.RUnlock()

	// The menu has to say which way it is going to flip the lever: an item that
	// just reads "GPU rendering" tells the user nothing about the current state,
	// and this one is the fix people reach for when the window is blank.
	gpuRenderingOff := gpuState != nil && gpuState()

	var sStatus, sAction, sShow, sPanic, sExit, sReport, sGPU string

	if lang == "en" {
		if conn {
			if lat > 0 {
				sStatus = fmt.Sprintf("🟢 Connected: %s (%dms)", pName, lat)
			} else if pName != "" {
				sStatus = fmt.Sprintf("🟢 Connected: %s", pName)
			} else {
				sStatus = "🟢 Connected (Protected)"
			}
			sAction = "🔴 Disconnect"
		} else {
			sStatus = "⚪ Status: Disconnected"
			sAction = "⚡ Fast Connect (Best Server)"
		}
		sShow = "🛡️ Show NexVPN"
		sPanic = "🚨 Emergency Panic Wipe"
		sExit = "🚪 Exit NexVPN"
		sReport = "📄 Open startup report"
		sGPU = "🖥️ Turn off GPU rendering (restart to apply)"
		if gpuRenderingOff {
			sGPU = "🖥️ Turn GPU rendering back on (restart to apply)"
		}
	} else {
		// Persian (fa)
		if conn {
			if lat > 0 {
				sStatus = fmt.Sprintf("🟢 متصل به: %s (%dms)", pName, lat)
			} else if pName != "" {
				sStatus = fmt.Sprintf("🟢 متصل به: %s", pName)
			} else {
				sStatus = "🟢 وضعیت: متصل و ایمن"
			}
			sAction = "🔴 قطع اتصال"
		} else {
			sStatus = "⚪ وضعیت: قطع اتصال"
			sAction = "⚡ اتصال سریع به بهترین سرور"
		}
		sShow = "🛡️ نمایش برنامه"
		sPanic = "🚨 پاکسازی و قطع اضطراری"
		sExit = "🚪 خروج از برنامه"
		sReport = "📄 باز کردن گزارش راه‌اندازی"
		sGPU = "🖥️ خاموش کردن رندر با کارت گرافیک (بعد از ری‌استارت)"
		if gpuRenderingOff {
			sGPU = "🖥️ روشن کردن رندر با کارت گرافیک (بعد از ری‌استارت)"
		}
	}

	ptrStatus, _ := syscall.UTF16PtrFromString(sStatus)
	ptrAction, _ := syscall.UTF16PtrFromString(sAction)
	ptrShow, _ := syscall.UTF16PtrFromString(sShow)
	ptrPanic, _ := syscall.UTF16PtrFromString(sPanic)
	ptrExit, _ := syscall.UTF16PtrFromString(sExit)

	// Status Header (Disabled item)
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING|MF_GRAYED|MF_DISABLED), uintptr(CMD_STATUS), uintptr(unsafe.Pointer(ptrStatus)))
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)

	// Primary Action (Connect / Disconnect)
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_TOGGLE_CONNECT), uintptr(unsafe.Pointer(ptrAction)))
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)

	// Servers submenu.
	//
	// TrackPopupMenuEx with TPM_RETURNCMD reports the id of the selected item
	// wherever it sits in the hierarchy, so a submenu needs nothing more than
	// unique ids. The map below is what turns the chosen id back into a profile
	// id, and it is deliberately a local: it only has to survive this call.
	trayMu.RLock()
	listFn := serverListFunc
	trayMu.RUnlock()

	serverSelection := make(map[uintptr]string)
	if listFn != nil {
		if servers := listFn(); len(servers) > 0 {
			hSub, _, _ := createPopupMenu.Call()
			if hSub != 0 {
				for i, server := range servers {
					// Pinned servers get a marker rather than a separate
					// section: a flat list stays scannable at a glance, which
					// is the entire point of a tray menu.
					label := server.Name
					if server.Favorite {
						label = "★ " + label
					}
					ptr, err := syscall.UTF16PtrFromString(label)
					if err != nil {
						continue
					}
					id := uintptr(CMD_SERVER_BASE + i)
					_, _, _ = appendMenuW.Call(hSub, uintptr(MF_STRING), id, uintptr(unsafe.Pointer(ptr)))
					serverSelection[id] = server.ID
				}

				sServers := "🖥️ Servers"
				if lang != "en" {
					sServers = "🖥️ سرورها"
				}
				ptrServers, _ := syscall.UTF16PtrFromString(sServers)
				_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_POPUP|MF_STRING), hSub, uintptr(unsafe.Pointer(ptrServers)))
				_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)
			}
		}
	}

	// Show Window
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_SHOW), uintptr(unsafe.Pointer(ptrShow)))

	// Panic Wipe
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_PANIC_WIPE), uintptr(unsafe.Pointer(ptrPanic)))

	// Diagnostics. Both are only listed when the app registered a handler: a menu
	// row that does nothing is worse than no row, and this menu is the only
	// surface a user with an undrawable window still has.
	if openReport != nil {
		ptrReport, err := syscall.UTF16PtrFromString(sReport)
		if err == nil {
			_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_OPEN_REPORT), uintptr(unsafe.Pointer(ptrReport)))
		}
	}
	if gpuToggle != nil {
		ptrGPU, err := syscall.UTF16PtrFromString(sGPU)
		if err == nil {
			_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_TOGGLE_GPU), uintptr(unsafe.Pointer(ptrGPU)))
		}
	}
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)

	// Exit
	_, _, _ = appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_EXIT), uintptr(unsafe.Pointer(ptrExit)))

	var pt POINT
	_, _, _ = getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	_, _, _ = setForegroundWin.Call(hWnd)
	cmd, _, _ := trackPopupMenuEx.Call(
		hMenu,
		uintptr(TPM_RETURNCMD|TPM_RIGHTBUTTON),
		uintptr(pt.X),
		uintptr(pt.Y),
		hWnd,
		0,
	)

	switch cmd {
	case CMD_SHOW:
		ShowMainWindow()
	case CMD_TOGGLE_CONNECT:
		trayMu.RLock()
		isConn := isConnected
		togFn := toggleConnectFunc
		fastFn := fastConnectFunc
		trayMu.RUnlock()

		if isConn {
			if togFn != nil {
				go togFn()
			}
		} else {
			if fastFn != nil {
				go fastFn()
			} else if togFn != nil {
				go togFn()
			}
		}
	case CMD_PANIC_WIPE:
		trayMu.RLock()
		pFn := panicWipeFunc
		trayMu.RUnlock()
		if pFn != nil {
			go pFn()
		}
	case CMD_OPEN_REPORT:
		trayMu.RLock()
		rFn := openReportFunc
		trayMu.RUnlock()
		if rFn != nil {
			go rFn()
		}
	case CMD_TOGGLE_GPU:
		trayMu.RLock()
		gFn := gpuToggleFunc
		trayMu.RUnlock()
		if gFn != nil {
			go gFn()
		}
	default:
		// Server submenu selection. The command id is an index into the map
		// built while the menu was assembled, so an id that is not in it can
		// only mean a stale menu — ignore it rather than guessing.
		if profileID, ok := serverSelection[cmd]; ok && profileID != "" {
			trayMu.RLock()
			selFn := serverSelectFunc
			trayMu.RUnlock()
			if selFn != nil {
				go selFn(profileID)
			}
		}
	case CMD_EXIT:
		trayMu.RLock()
		eFn := exitFunc
		trayMu.RUnlock()
		RemoveSystemTray()
		if eFn != nil {
			// eFn is runtime.Quit(ctx): it triggers Wails' OnShutdown, which runs
			// app.shutdown() -> engine.Disconnect(), sysproxy.ClearSystemProxy() and
			// KillSwitch.Disable(). That cleanup takes seconds (core teardown, netsh
			// calls, DNS flush), so it must run in the background - calling it
			// synchronously from the window procedure would block the message loop.
			go eFn()
		}
		// Safety net only. Previously this forced os.Exit(0) after just 150ms,
		// which routinely killed the process in the middle of the cleanup and left
		// users with a stale system proxy or an active kill-switch firewall policy
		// (i.e. no internet at all). Wait long enough for a graceful shutdown and
		// force-exit only if it genuinely hangs.
		go func() {
			time.Sleep(15 * time.Second)
			os.Exit(0)
		}()
	}
}

// ApplyMainWindowIcon continuously searches for the Wails desktop window on launch
func ApplyMainWindowIcon() {
	go func() {
		hBig := getBestHIcon(256, 256)
		hSmall := getBestHIcon(32, 32)

		wndTitle, _ := syscall.UTF16PtrFromString("NexVPN")

		// Poll for the window creation during startup (up to 10 seconds)
		for i := 0; i < 50; i++ {
			time.Sleep(200 * time.Millisecond)
			hWnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(wndTitle)))
			if hWnd != 0 {
				idxBig := int32(-14)
				idxSmall := int32(-34)
				if hBig != 0 {
					_, _, _ = sendMessageW.Call(hWnd, uintptr(WM_SETICON), uintptr(ICON_BIG), hBig)
					if setClassLongPtrW.Find() == nil {
						_, _, _ = setClassLongPtrW.Call(hWnd, uintptr(uint32(idxBig)), hBig)
					}
				}
				if hSmall != 0 {
					_, _, _ = sendMessageW.Call(hWnd, uintptr(WM_SETICON), uintptr(ICON_SMALL), hSmall)
					if setClassLongPtrW.Find() == nil {
						_, _, _ = setClassLongPtrW.Call(hWnd, uintptr(uint32(idxSmall)), hSmall)
					}
				}
				break
			}
		}
	}()
}

// InitSystemTray initializes the notification area icon near the clock with right-click menu
func InitSystemTray(tooltip string) {
	trayMu.RLock()
	alreadyInit := activeNID != nil
	trayMu.RUnlock()
	if alreadyInit {
		return
	}

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hInst, _, _ := getModuleHandleW.Call(0)
		className, _ := syscall.UTF16PtrFromString("NexVPNTrayWindowClass")

		hIcon := getBestHIcon(32, 32)

		var wcx WNDCLASSEXW
		wcx.CbSize = uint32(unsafe.Sizeof(wcx))
		wcx.LpfnWndProc = syscall.NewCallback(trayWndProc)
		wcx.HInstance = hInst
		wcx.LpszClassName = className
		wcx.HIcon = hIcon
		wcx.HIconSm = hIcon

		_, _, _ = registerClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

		wndTitle, _ := syscall.UTF16PtrFromString("NexVPNTrayMsgWindow")
		hWnd, _, _ := createWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(wndTitle)),
			0, 0, 0, 0, 0,
			0, 0, hInst, 0,
		)

		trayMu.Lock()
		trayHWnd = hWnd
		trayMu.Unlock()

		var nid NOTIFYICONDATAW
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.HWnd = hWnd
		nid.UID = 1001
		nid.UFlags = NIF_ICON | NIF_TIP | NIF_MESSAGE
		nid.UCallbackMessage = WM_TRAYCALLBACK
		nid.HIcon = hIcon

		tip16, _ := syscall.UTF16FromString(tooltip)
		copy(nid.SzTip[:], tip16)

		ret, _, _ := shellNotifyIconW.Call(uintptr(NIM_ADD), uintptr(unsafe.Pointer(&nid)))
		if ret != 0 {
			trayMu.Lock()
			activeNID = &nid
			trayMu.Unlock()
		}

		// Message loop for tray window
		var msg MSG
		for {
			r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || r == uintptr(^uintptr(0)) {
				break
			}
			_, _, _ = translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			_, _, _ = dispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

// ShowMainWindow finds the NexVPN window and brings it to the foreground
func ShowMainWindow() {
	if !atomic.CompareAndSwapUint32(&isShowingAtomic, 0, 1) {
		return
	}
	defer atomic.StoreUint32(&isShowingAtomic, 0)

	if showFunc != nil {
		showFunc()
	}
	wndTitle, _ := syscall.UTF16PtrFromString("NexVPN")
	hWnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(wndTitle)))
	if hWnd != 0 {
		_, _, _ = showWindow.Call(hWnd, uintptr(SW_RESTORE))
		_, _, _ = showWindow.Call(hWnd, uintptr(SW_SHOW))
		_, _, _ = setForegroundWin.Call(hWnd)
	}
}

// RemoveSystemTray removes the icon when the app shuts down
func RemoveSystemTray() {
	trayMu.Lock()
	nid := activeNID
	activeNID = nil
	hWnd := trayHWnd
	trayHWnd = 0
	trayMu.Unlock()

	if nid != nil {
		_, _, _ = shellNotifyIconW.Call(uintptr(NIM_DELETE), uintptr(unsafe.Pointer(nid)))
	}
	if hWnd != 0 {
		_, _, _ = destroyWindow.Call(hWnd)
	}
}

// UpdateTooltip updates the tooltip text on the tray icon
func UpdateTooltip(tooltip string) {
	trayMu.Lock()
	defer trayMu.Unlock()
	if activeNID != nil {
		tip16, _ := syscall.UTF16FromString(tooltip)
		copy(activeNID.SzTip[:], tip16)
		_, _, _ = shellNotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(activeNID)))
	}
}
