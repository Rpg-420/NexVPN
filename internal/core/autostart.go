package core

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const runRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appValueName = "NexVPN"
const taskName = "NexVPN_AutoStart"

// SetAutoStart configures elevated Windows autostart via Task Scheduler with registry fallback
func SetAutoStart(enable bool) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	if enable {
		// 1. Try creating an elevated Task in Task Scheduler (starts with highest privileges, no UAC on login)
		cmdTask := exec.Command("schtasks", "/Create", "/F", "/TN", taskName, "/TR", fmt.Sprintf(`"%s" --minimized`, execPath), "/SC", "ONLOGON", "/RL", "HIGHEST")
		cmdTask.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
		if err := cmdTask.Run(); err == nil {
			// Clean registry entry if task succeeded to prevent double launch
			cleanRegistryRun()
			return nil
		}

		// 2. Fallback: Registry Run Key
		k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(appValueName, `"`+execPath+`" --minimized`)
	} else {
		// Remove Task Scheduler task
		cmdDel := exec.Command("schtasks", "/Delete", "/F", "/TN", taskName)
		cmdDel.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
		_ = cmdDel.Run()

		// Remove Registry Run key
		cleanRegistryRun()
		return nil
	}
}

func cleanRegistryRun() {
	if k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE); err == nil {
		_ = k.DeleteValue(appValueName)
		k.Close()
	}
}
