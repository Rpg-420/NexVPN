package main

import (
	"embed"
	"os"
	"os/signal"
	"syscall"

	"nexvpn/internal/core"
	"nexvpn/internal/sysproxy"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// Graceful Shutdown & Signal Trap:
	// Ensures proxy and firewall rules are always restored even on force termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		if app.engine != nil {
			_ = app.engine.Disconnect()
		}
		sysproxy.ClearSystemProxy()
		_ = core.GetKillSwitch().Disable()
		core.CleanupStaleCores()
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
		BackgroundColour: &options.RGBA{R: 8, G: 10, B: 15, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windows.Dark,
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
	sysproxy.ClearSystemProxy()
	_ = core.GetKillSwitch().Disable()
	core.CleanupStaleCores()

	if err != nil {
		println("Error:", err.Error())
	}
}

