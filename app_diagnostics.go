package main

import (
	"fmt"
	"os/exec"
	"strings"

	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Everything in this file answers one question: what does a user do when the
// window itself is the thing that is broken?
//
// A Wails app renders into WebView2, and WebView2 can fail to present for reasons
// the Go side cannot observe — a driver that composites nothing, or an
// endpoint-security product that injects an unsigned DLL and gets the renderer
// killed. In both cases the process is healthy, the tunnel may even be up, and
// every control that could fix it lives inside a window nobody can see. The tray
// icon and a Win32 dialog are the two surfaces that still work, so both are used
// here, and neither goes through the frontend.

// registerTrayDiagnostics adds the diagnostics rows to the tray menu. Called
// from startup, next to the other tray wiring.
func (a *App) registerTrayDiagnostics() {
	tray.SetDiagnosticsHandlers(a.openStartupReport, a.ToggleWebviewRendering, a.WebviewGpuDisabled)
}

// onSecondInstance handles a second launch of the app.
//
// Two instances cannot coexist honestly: both bind 10808/10809, both write the
// same config.json, and the second one's kill-switch rules silently replace the
// first one's. The second process now forwards its intent and this one raises the
// window the user was looking for.
func (a *App) onSecondInstance(data options.SecondInstanceData) {
	diary.Log("launch", "second-instance-suppressed", "args", strings.Join(data.Args, " "))

	if ctx := a.appCtx(); ctx != nil {
		runtime.WindowUnminimise(ctx)
		runtime.WindowShow(ctx)
	}
	tray.ShowMainWindow()
}

// openStartupReport shows the launch diary in Notepad.
//
// The file is the same one tools/NexVPN-Report.cmd reads; opening it from the
// tray is the shortcut for someone whose window never drew.
func (a *App) openStartupReport() {
	path := diary.Path()
	if path == "" {
		sysproc.ShowNoticeDialog("NexVPN", "مسیر گزارش راه‌اندازی در دسترس نیست.")
		return
	}

	// Notepad is the point: it renders the tab-separated diary readably and works
	// with no association configured for a .log file.
	cmd := exec.Command("notepad.exe", path)
	if err := cmd.Start(); err != nil {
		diary.Log("diagnostics", "report-open-failed", "path", path, "err", err.Error())
		sysproc.ShowFatalDialog("NexVPN", fmt.Sprintf("گزارش راه‌اندازی باز نشد:\n%s\n\n%v", path, err))
		return
	}
	diary.Log("diagnostics", "report-opened", "path", path)
}

// WebviewGpuDisabled reports the persisted rendering mode, so the tray menu can
// label its item with the action it is about to take.
func (a *App) WebviewGpuDisabled() bool {
	if a.store == nil {
		return false
	}
	return a.store.GetSettings().WebviewDisableGpu
}

// ToggleWebviewRendering flips WebView2's GPU acceleration.
//
// The change only applies to the next process start — WebView2 takes the flag
// when the window is created — so the dialog says that out loud instead of
// leaving a toggle that appears to do nothing.
func (a *App) ToggleWebviewRendering() {
	if a.store == nil {
		return
	}

	settings := a.store.GetSettings()
	settings.WebviewDisableGpu = !settings.WebviewDisableGpu
	if err := a.store.SaveSettings(settings); err != nil {
		diary.LogErr("diagnostics", "webview-gpu-save-failed", err)
		sysproc.ShowFatalDialog("NexVPN", fmt.Sprintf("تنظیمات رندر ذخیره نشد:\n%v", err))
		return
	}

	diary.Log("diagnostics", "webview-gpu-toggled", "disabled", fmt.Sprintf("%t", settings.WebviewDisableGpu))
	state := "خاموش"
	if !settings.WebviewDisableGpu {
		state = "روشن"
	}
	sysproc.ShowNoticeDialog("NexVPN", fmt.Sprintf(
		"رندر با کارت گرافیک %s شد.\n\nبرای اعمال، برنامه را کامل ببندید (از آیکون سینی → خروج) و دوباره باز کنید.", state))
}
