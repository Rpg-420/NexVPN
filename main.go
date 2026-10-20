package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"nexvpn/internal/config"
	"nexvpn/internal/core"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
	"nexvpn/internal/sysproxy"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// The launch diary comes up before anything that can fail. If NewApp panics
	// or wails.Run never returns, this file is the only surviving record of how
	// far the launch got — which is exactly the situation the standalone
	// NexVPN-Report tool exists to diagnose.
	_ = diary.Init(config.DefaultDataDir(), AppVersion)
	defer diary.Close()
	diary.Log("launch", "main-enter")

	app := NewApp()
	diary.Log("launch", "app-constructed")

	// Read once, here: WebView2 takes the rendering options when the window is
	// created and has no way to change them afterwards, so this is the only place
	// that can honour them.
	startupSettings := app.GetSettings()
	diary.Log("launch", "rendering-policy",
		"gpu_disabled", fmt.Sprintf("%t", startupSettings.WebviewDisableGpu),
		"code_integrity_disabled", fmt.Sprintf("%t", startupSettings.WebviewDisableCodeIntegrity))

	// Graceful Shutdown & Signal Trap:
	// Ensures proxy and firewall rules are always restored even on force termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		diary.Log("launch", "signal-received")
		if app.engine != nil {
			_ = app.engine.Disconnect()
		}
		_ = sysproxy.ClearSystemProxy()
		_ = core.GetKillSwitch().ForceCleanup()
		core.CleanupStaleCores()
		diary.Close()
		os.Exit(0)
	}()

	err := wails.Run(&options.App{
		Title:     "NexVPN",
		Width:     1120,
		Height:    720,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 8, G: 10, B: 15, A: 1},
		OnStartup:         app.startup,
		OnShutdown:        app.shutdown,
		HideWindowOnClose: true,
		// One instance, because two of them cannot coexist honestly: both bind
		// 10808/10809, both write the same config.json, and the second one's
		// kill-switch rules override the first one's. A second launch now raises
		// the window the user was looking for instead of starting a fight.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.nexvpn.desktop",
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			// Read once, before the webview exists: the tray's rendering lever is
			// deliberately labelled "restart to apply" rather than pretending a
			// live switch is possible. Both default to false, which is the
			// behaviour every existing install has.
			WebviewGpuIsDisabled:                startupSettings.WebviewDisableGpu,
			WebviewDisableRendererCodeIntegrity: startupSettings.WebviewDisableCodeIntegrity,
			Theme:                               windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:   windows.RGB(8, 10, 15),
				DarkModeTitleText:  windows.RGB(255, 255, 255),
				DarkModeBorder:     windows.RGB(30, 30, 46),
				LightModeTitleBar:  windows.RGB(8, 10, 15),
				LightModeTitleText: windows.RGB(255, 255, 255),
				LightModeBorder:    windows.RGB(30, 30, 46),
			},
		},
		EnableDefaultContextMenu: false,
	})

	// Ensure cleanup on normal termination
	if app.engine != nil {
		_ = app.engine.Disconnect()
	}
	_ = sysproxy.ClearSystemProxy()
	_ = core.GetKillSwitch().ForceCleanup()
	core.CleanupStaleCores()

	if err != nil {
		diary.LogErr("launch", "wails-exited-with-error", err)
		println("Error:", err.Error())
		// The window may be exactly what failed, so the reason cannot live only
		// inside it. This is the last thing that can still reach the user.
		sysproc.ShowFatalDialog("NexVPN", fmt.Sprintf(
			"برنامه غیرمنتظره بسته شد:\n\n%v\n\nگزارش راه‌اندازی:\n%s",
			err, diary.Path()))
	} else {
		diary.Log("launch", "wails-exited")
	}
}
