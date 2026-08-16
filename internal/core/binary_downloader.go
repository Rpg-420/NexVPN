package core

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// BinaryInfo holds status of external core binaries
type BinaryInfo struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Status      string `json:"status"` // installed, missing
	Path        string `json:"path"`
	DownloadURL string `json:"download_url"`
}

// Direct release zip URLs for 1-click automatic download (Windows amd64)
var directDownloadURLs = map[string]string{
	"xray":      "https://github.com/XTLS/Xray-core/releases/download/v1.8.24/Xray-windows-64.zip",
	"singbox":   "https://github.com/SagerNet/sing-box/releases/download/v1.9.7/sing-box-1.9.7-windows-amd64.zip",
	"openvpn":   "https://github.com/portapps/openvpn-portable/releases/download/2.5.8-19/openvpn-portable-win64.zip",
	"wireguard": "https://www.wintun.net/builds/wintun-0.14.1.zip",
}

// CheckBinaries checks availability of all protocol core binaries
func CheckBinaries() map[string]BinaryInfo {
	binaries := map[string]BinaryInfo{
		"xray": {
			Name:        "Xray-core (VLESS, VMess, Trojan, Reality)",
			Filename:    exeName("xray"),
			DownloadURL: directDownloadURLs["xray"],
		},
		"singbox": {
			Name:        "Sing-box (WARP, Shadowsocks, Hy2, TUN)",
			Filename:    exeName("sing-box"),
			DownloadURL: directDownloadURLs["singbox"],
		},
		"wireguard": {
			Name:        "Wintun Virtual Adapter Driver",
			Filename:    dllName("wintun"),
			DownloadURL: directDownloadURLs["wireguard"],
		},
		"openvpn": {
			Name:        "OpenVPN Core Client",
			Filename:    exeName("openvpn"),
			DownloadURL: directDownloadURLs["openvpn"],
		},
	}

	execDir, _ := os.Executable()
	execFolder := filepath.Dir(execDir)
	cwd, _ := os.Getwd()

	searchPaths := []string{
		filepath.Join(execFolder, "bin"),
		filepath.Join(execFolder, "tools"),
		filepath.Join(execFolder, "bin", "bin"),
		filepath.Join(cwd, "build", "bin", "bin"),
		filepath.Join(cwd, "build", "bin"),
		filepath.Join(cwd, "bin"),
		filepath.Join(cwd, "tools"),
		`C:\Program Files\OpenVPN\bin`,
		`C:\Program Files (x86)\OpenVPN\bin`,
		".",
		"./bin",
		"./build/bin/bin",
		"./build/bin",
	}

	for key, info := range binaries {
		foundPath := ""

		// Check system PATH
		if p, err := exec.LookPath(info.Filename); err == nil {
			foundPath = p
		} else {
			// Check local search paths
			for _, folder := range searchPaths {
				target := filepath.Join(folder, info.Filename)
				if _, err := os.Stat(target); err == nil {
					foundPath = target
					break
				}
			}
		}

		if foundPath != "" {
			info.Status = "installed"
			info.Path = foundPath
		} else {
			info.Status = "missing"
		}

		binaries[key] = info
	}

	return binaries
}

// DownloadAndInstallCore downloads and extracts a core binary directly into bin/ directory
func DownloadAndInstallCore(coreKey string) error {
	downloadURL, ok := directDownloadURLs[coreKey]
	if !ok || downloadURL == "" {
		return fmt.Errorf("no direct download URL available for %s", coreKey)
	}

	execDir, _ := os.Executable()
	binDir := filepath.Join(filepath.Dir(execDir), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) NexVPN/1.3")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP status: %s", resp.Status)
	}

	// If direct executable
	if strings.HasSuffix(downloadURL, ".exe") {
		destPath := filepath.Join(binDir, exeName(coreKey))
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, resp.Body)
		return err
	}

	// ZIP file extraction
	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read zip payload: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("invalid zip file: %w", err)
	}

	extractedCount := 0
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		baseName := filepath.Base(file.Name)
		ext := strings.ToLower(filepath.Ext(baseName))

		// Extract executables, DLLs, and routing data files (.dat / .db)
		if ext == ".exe" || ext == ".dll" || ext == ".dat" || ext == ".db" || ext == ".sys" {
			rc, err := file.Open()
			if err != nil {
				continue
			}

			destPath := filepath.Join(binDir, baseName)
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				continue
			}

			_, _ = io.Copy(out, rc)
			out.Close()
			rc.Close()
			extractedCount++
		}
	}

	if extractedCount == 0 {
		return fmt.Errorf("no executable or data files found in zip payload")
	}

	return nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func dllName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".dll"
	}
	return name + ".so"
}
