//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"

	"nexvpn/internal/sysproc"

	"golang.org/x/sys/windows/registry"
)

const runRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appValueName = "NexVPN"
const taskName = "NexVPN_AutoStart"

// SetAutoStart configures elevated Windows autostart via Task Scheduler with registry fallback
func SetAutoStart(enable bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	if enable {
		// 1. Try creating an elevated Task in Task Scheduler (starts with highest privileges, no UAC on login)
		cmdTask := exec.Command("schtasks", "/Create", "/F", "/TN", taskName, "/TR", fmt.Sprintf(`\"%s\" --minimized`, execPath), "/SC", "ONLOGON", "/RL", "HIGHEST")
		cmdTask.SysProcAttr = sysproc.HideWindowAttr()
		if err := cmdTask.Run(); err == nil {
			cleanRegistryRun()
			return nil
		}

		// 2. Fallback: Registry Run Key
		k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer func() { _ = k.Close() }()
		return k.SetStringValue(appValueName, `"`+execPath+`" --minimized`)
	} else {
		// Remove Task Scheduler task
		cmdDel := exec.Command("schtasks", "/Delete", "/F", "/TN", taskName)
		cmdDel.SysProcAttr = sysproc.HideWindowAttr()
		_ = cmdDel.Run()

		// Remove Registry Run key
		cleanRegistryRun()
		return nil
	}
}

func cleanRegistryRun() {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err == nil {
		_ = k.DeleteValue(appValueName)
		_ = k.Close()
	}
}
