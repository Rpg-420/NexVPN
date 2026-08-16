//go:build windows

package tray

import (
	_ "embed"
	"os"
	"runtime"
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
	postQuitMessage          = user32.NewProc("PostQuitMessage")
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

	GCLP_HICON   = int32(-14)
	GCLP_HICONSM = int32(-34)

	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	MF_STRING       = 0x00000000
	MF_SEPARATOR    = 0x00000800
	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTBUTTON = 0x0002

	SW_RESTORE = 9
	SW_SHOW    = 5

	IDI_APPLICATION = 32512

	CMD_SHOW       = 1001
	CMD_DISCONNECT = 1002
	CMD_EXIT       = 1003
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
	activeNID      *NOTIFYICONDATAW
	trayHWnd       uintptr
	disconnectFunc func()
)

// SetDisconnectHandler registers a callback invoked when Disconnect is clicked from tray menu
func SetDisconnectHandler(fn func()) {
	disconnectFunc = fn
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
			extractIconExW.Call(
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
	defer destroyMenu.Call(hMenu)

	sShow, _ := syscall.UTF16PtrFromString("🛡️ نمایش برنامه (Show NexVPN)")
	sDisc, _ := syscall.UTF16PtrFromString("🔴 قطع اتصال (Disconnect)")
	sExit, _ := syscall.UTF16PtrFromString("🚪 خروج (Exit NexVPN)")

	appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_SHOW), uintptr(unsafe.Pointer(sShow)))
	appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)
	appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_DISCONNECT), uintptr(unsafe.Pointer(sDisc)))
	appendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)
	appendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(CMD_EXIT), uintptr(unsafe.Pointer(sExit)))

	var pt POINT
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	setForegroundWin.Call(hWnd)
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
	case CMD_DISCONNECT:
		if disconnectFunc != nil {
			go disconnectFunc()
		}
	case CMD_EXIT:
		RemoveSystemTray()
		os.Exit(0)
	}
}

// ApplyMainWindowIcon continuously searches for the Wails desktop window on launch
// and forcefully sets the native Windows taskbar icon via WM_SETICON (big and small)
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
					sendMessageW.Call(hWnd, uintptr(WM_SETICON), uintptr(ICON_BIG), hBig)
					if setClassLongPtrW.Find() == nil {
						setClassLongPtrW.Call(hWnd, uintptr(uint32(idxBig)), hBig)
					}
				}
				if hSmall != 0 {
					sendMessageW.Call(hWnd, uintptr(WM_SETICON), uintptr(ICON_SMALL), hSmall)
					if setClassLongPtrW.Find() == nil {
						setClassLongPtrW.Call(hWnd, uintptr(uint32(idxSmall)), hSmall)
					}
				}
				break
			}
		}
	}()
}

// InitSystemTray initializes the notification area icon near the clock with right-click menu
func InitSystemTray(tooltip string) {
	if activeNID != nil {
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

		registerClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

		wndTitle, _ := syscall.UTF16PtrFromString("NexVPNTrayMsgWindow")
		hWnd, _, _ := createWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(wndTitle)),
			0, 0, 0, 0, 0,
			0, 0, hInst, 0,
		)

		trayHWnd = hWnd

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
			activeNID = &nid
		}

		// Message loop for tray window
		var msg MSG
		for {
			r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || r == uintptr(^uintptr(0)) {
				break
			}
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

// ShowMainWindow finds the NexVPN window and brings it to the foreground
func ShowMainWindow() {
	wndTitle, _ := syscall.UTF16PtrFromString("NexVPN")
	hWnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(wndTitle)))
	if hWnd != 0 {
		showWindow.Call(hWnd, uintptr(SW_RESTORE))
		setForegroundWin.Call(hWnd)
	}
}

// RemoveSystemTray removes the icon when the app shuts down
func RemoveSystemTray() {
	if activeNID != nil {
		shellNotifyIconW.Call(uintptr(NIM_DELETE), uintptr(unsafe.Pointer(activeNID)))
		activeNID = nil
	}
	if trayHWnd != 0 {
		destroyWindow.Call(trayHWnd)
		trayHWnd = 0
	}
}

// UpdateTooltip updates the tooltip text on the tray icon
func UpdateTooltip(tooltip string) {
	if activeNID != nil {
		tip16, _ := syscall.UTF16FromString(tooltip)
		copy(activeNID.SzTip[:], tip16)
		shellNotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(activeNID)))
	}
}
