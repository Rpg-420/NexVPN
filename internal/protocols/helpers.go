package protocols

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"nexvpn/internal/sysproc"
)

type LoggerFunc func(level, message, source string)

// PrepareHiddenCmd ensures that on Windows no console / CMD window ever pops up
func PrepareHiddenCmd(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = sysproc.HideWindowAttr()
	return cmd
}

// KillProcessTree terminates a process tree without popping any CMD window
func KillProcessTree(pid int) {
	sysproc.KillProcessTree(pid)
}

func findBinary(name string) (string, error) {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		name = name + ".exe"
	}

	execDir, _ := os.Executable()
	execFolder := filepath.Dir(execDir)
	cwd, _ := os.Getwd()

	searchFolders := []string{
		filepath.Join(execFolder, "bin"),
		filepath.Join(execFolder, "..", "bin"),
		filepath.Join(execFolder, "..", "..", "bin"),
		execFolder,
		filepath.Join(cwd, "bin"),
		filepath.Join(cwd, "build", "bin"),
		filepath.Join(cwd, "build", "bin", "bin"),
		cwd,
		`C:\Program Files\OpenVPN\bin`,
		`C:\Program Files (x86)\OpenVPN\bin`,
		`C:\Program Files\Sing-box`,
		`C:\Program Files\Xray`,
	}

	for _, folder := range searchFolders {
		target := filepath.Join(folder, name)
		if abs, err := filepath.Abs(target); err == nil {
			if info, statErr := os.Stat(abs); statErr == nil && !info.IsDir() {
				return abs, nil
			}
		}
	}

	if lp, err := exec.LookPath(name); err == nil {
		if abs, err := filepath.Abs(lp); err == nil {
			return abs, nil
		}
		return lp, nil
	}

	return "", fmt.Errorf("binary %s not found in any search path", name)
}

// hasNetCap checks if a binary has CAP_NET_ADMIN capabilities
func hasNetCap(binPath string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	
	getcapBin, err := exec.LookPath("getcap")
	if err != nil {
		if _, errSbin := os.Stat("/sbin/getcap"); errSbin == nil {
			getcapBin = "/sbin/getcap"
		} else if _, errUsrSbin := os.Stat("/usr/sbin/getcap"); errUsrSbin == nil {
			getcapBin = "/usr/sbin/getcap"
		} else {
			return false
		}
	}

	out, err := exec.Command(getcapBin, binPath).Output()
	if err != nil {
		return false
	}
	outStr := string(out)
	return strings.Contains(outStr, "cap_net_admin") || strings.Contains(outStr, "CAP_NET_ADMIN")
}
