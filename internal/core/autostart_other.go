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

	return nil
}
