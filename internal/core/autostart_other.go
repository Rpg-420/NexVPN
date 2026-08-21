//go:build !windows

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SetAutoStart configures autostart on Linux (.config/autostart) and macOS
func SetAutoStart(enable bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if runtime.GOOS == "linux" {
		autostartDir := filepath.Join(homeDir, ".config", "autostart")
		desktopFile := filepath.Join(autostartDir, "nexvpn.desktop")

		if enable {
			_ = os.MkdirAll(autostartDir, 0755)
			content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=NexVPN
Exec=%s --minimized
Icon=nexvpn
Comment=NexVPN Desktop Client
X-GNOME-Autostart-enabled=true
`, execPath)
			return os.WriteFile(desktopFile, []byte(content), 0644)
		} else {
			_ = os.Remove(desktopFile)
			return nil
		}
	}

	if runtime.GOOS == "darwin" {
		launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
		plistFile := filepath.Join(launchAgentsDir, "dev.nexvpn.plist")

		if enable {
			_ = os.MkdirAll(launchAgentsDir, 0755)
			content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.nexvpn</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--minimized</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>`, execPath)
			return os.WriteFile(plistFile, []byte(content), 0644)
		} else {
			_ = os.Remove(plistFile)
			return nil
		}
	}

	return nil
}
