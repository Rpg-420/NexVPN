package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"nexvpn/internal/sysproc"
)

// BinaryInfo holds status of external core binaries
type BinaryInfo struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Status      string `json:"status"` // installed, missing
	Path        string `json:"path"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

// GetCoreDownloadURL returns official release URLs tailored to the current OS and CPU architecture
func GetCoreDownloadURL(coreKey string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch coreKey {
	case "xray":
		switch {
		case osName == "windows" && arch == "amd64":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"
		case osName == "windows" && arch == "386":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-32.zip"
		case osName == "windows" && arch == "arm64":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-arm64-v8a.zip"
		case osName == "linux" && arch == "amd64":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"
		case osName == "linux" && arch == "arm64":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip"
		case osName == "darwin" && arch == "arm64":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-macos-arm64-v8a.zip"
		case osName == "darwin":
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-macos-64.zip"
		default:
			return "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"
		}

	case "singbox":
		switch {
		case osName == "windows" && arch == "amd64":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-windows-amd64.zip"
		case osName == "windows" && arch == "arm64":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-windows-arm64.zip"
		case osName == "linux" && arch == "amd64":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-linux-amd64.tar.gz"
		case osName == "linux" && arch == "arm64":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-linux-arm64.tar.gz"
		case osName == "darwin" && arch == "arm64":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-darwin-arm64.tar.gz"
		case osName == "darwin":
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-darwin-amd64.tar.gz"
		default:
			return "https://github.com/SagerNet/sing-box/releases/download/v1.10.7/sing-box-1.10.7-windows-amd64.zip"
		}

	case "geoip":
		return "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	case "geosite":
		return "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
	case "aether":
		switch {
		case osName == "windows" && arch == "amd64":
			return "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-windows-x86_64.zip"
		case osName == "linux" && arch == "amd64":
			return "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-linux-x86_64.tar.gz"
		case osName == "linux" && arch == "arm64":
			return "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-linux-arm64.tar.gz"
		case osName == "darwin":
			return "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-macos-x86_64.tar.gz"
		default:
			return "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-windows-x86_64.zip"
		}
	}

	return ""
}

// CheckBinaries checks availability and versions of all core engines
func CheckBinaries() map[string]BinaryInfo {
	binaries := map[string]BinaryInfo{
		"xray": {
			Name:        "Xray-core (VLESS Reality, VMess, Trojan)",
			Filename:    exeName("xray"),
			DownloadURL: GetCoreDownloadURL("xray"),
		},
		"singbox": {
			Name:        "Sing-box Universal Engine (WARP, TUN, DPI Bypass)",
			Filename:    exeName("sing-box"),
			DownloadURL: GetCoreDownloadURL("singbox"),
		},
		"aether": {
			Name:        "Aether Evasion Engine (MASQUE, gool, scan modes)",
			Filename:    exeName("aether"),
			DownloadURL: GetCoreDownloadURL("aether"),
		},
		"openvpn": {
			Name:        "OpenVPN Native Client",
			Filename:    exeName("openvpn"),
			DownloadURL: "https://github.com/portapps/openvpn-portable/releases/download/2.5.8-19/openvpn-portable-win64.zip",
		},
		"geoip": {
			Name:        "GeoIP Routing Database (Country IPs)",
			Filename:    "geoip.dat",
			DownloadURL: GetCoreDownloadURL("geoip"),
		},
		"geosite": {
			Name:        "GeoSite Routing Database (Ads, Malware, Domains)",
			Filename:    "geosite.dat",
			DownloadURL: GetCoreDownloadURL("geosite"),
		},
	}

	if runtime.GOOS == "windows" {
		binaries["wireguard"] = BinaryInfo{
			Name:        "Wintun Kernel Network Adapter",
			Filename:    dllName("wintun"),
			DownloadURL: "https://www.wintun.net/builds/wintun-0.14.1.zip",
		}
	}

	execDir, _ := os.Executable()
	execFolder := filepath.Dir(execDir)
	cwd, _ := os.Getwd()

	searchPaths := []string{
		filepath.Join(execFolder, "bin"),
		filepath.Join(cwd, "bin"),
		filepath.Join(cwd, "build", "bin", "bin"),
		filepath.Join(cwd, "build", "bin"),
		filepath.Join(execFolder, "..", "bin"),
		execFolder,
		cwd,
	}

	for key, info := range binaries {
		foundPath := ""

		for _, folder := range searchPaths {
			target := filepath.Join(folder, info.Filename)
			if _, err := os.Stat(target); err == nil {
				foundPath = target
				break
			}
		}

		if foundPath == "" {
			if p, err := exec.LookPath(info.Filename); err == nil {
				foundPath = p
			}
		}

		if foundPath != "" {
			info.Status = "installed"
			info.Path = foundPath
			info.Version = queryCoreVersion(key, foundPath)
		} else {
			info.Status = "missing"
			info.Version = "Not Installed"
		}

		binaries[key] = info
	}

	return binaries
}

func queryCoreVersion(coreKey, binPath string) string {
	if coreKey == "wireguard" {
		return "v0.14.1 (Wintun Driver)"
	}
	if coreKey == "geoip" || coreKey == "geosite" {
		if info, err := os.Stat(binPath); err == nil {
			return info.ModTime().Format("2006-01-02")
		}
		return "Installed"
	}

	var cmd *exec.Cmd
	switch coreKey {
	case "singbox":
		cmd = exec.Command(binPath, "version")
	case "xray":
		cmd = exec.Command(binPath, "version")
	case "openvpn":
		cmd = exec.Command(binPath, "--version")
	case "aether":
		return "v1.7.0"
	default:
		return "Installed"
	}

	cmd.SysProcAttr = sysproc.HideWindowAttr()
	out, err := cmd.Output()
	if err != nil {
		return "Installed"
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if len(firstLine) > 40 {
			return firstLine[:40]
		}
		return firstLine
	}
	return "Installed"
}

// UpdateCoreEngine downloads and hot-updates an engine binary
func UpdateCoreEngine(coreKey string) (string, error) {
	url := GetCoreDownloadURL(coreKey)
	if url == "" {
		return "", fmt.Errorf("no download URL available for %s", coreKey)
	}

	targetDir := getBinDirectory()
	_ = os.MkdirAll(targetDir, 0755)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 NexVPN-Updater/1.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download core: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server responded with status %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Determine archive type
	if strings.HasSuffix(url, ".zip") {
		err = extractZip(data, targetDir, coreKey)
	} else if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		err = extractTarGz(data, targetDir, coreKey)
	} else {
		// Direct file
		dest := filepath.Join(targetDir, exeName(coreKey))
		err = os.WriteFile(dest, data, 0755)
	}

	if err != nil {
		return "", fmt.Errorf("extraction error: %w", err)
	}

	return fmt.Sprintf("Successfully updated %s to latest version", coreKey), nil
}

// UpdateGeoData downloads and updates GeoIP and GeoSite routing databases
func UpdateGeoData() (string, error) {
	binDir := getBinDirectory()
	_ = os.MkdirAll(binDir, 0755)

	client := &http.Client{Timeout: 60 * time.Second}

	// 1. GeoIP
	respIP, err := client.Get(GetCoreDownloadURL("geoip"))
	if err == nil && respIP.StatusCode == http.StatusOK {
		defer respIP.Body.Close()
		outIP, err := os.Create(filepath.Join(binDir, "geoip.dat"))
		if err == nil {
			_, _ = io.Copy(outIP, respIP.Body)
			outIP.Close()
		}
	}

	// 2. GeoSite
	respSite, err := client.Get(GetCoreDownloadURL("geosite"))
	if err == nil && respSite.StatusCode == http.StatusOK {
		defer respSite.Body.Close()
		outSite, err := os.Create(filepath.Join(binDir, "geosite.dat"))
		if err == nil {
			_, _ = io.Copy(outSite, respSite.Body)
			outSite.Close()
		}
	}

	return "GeoIP and GeoSite routing rules updated successfully!", nil
}

func getBinDirectory() string {
	execDir, _ := os.Executable()
	baseDir := filepath.Dir(execDir)

	// Check if bin exists in baseDir
	if info, err := os.Stat(filepath.Join(baseDir, "bin")); err == nil && info.IsDir() {
		return filepath.Join(baseDir, "bin")
	}

	cwd, _ := os.Getwd()
	if info, err := os.Stat(filepath.Join(cwd, "bin")); err == nil && info.IsDir() {
		return filepath.Join(cwd, "bin")
	}

	return filepath.Join(baseDir, "bin")
}

func extractZip(data []byte, targetDir, coreKey string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	targetExe := exeName(coreKey)
	if coreKey == "singbox" {
		targetExe = exeName("sing-box")
	}

	for _, file := range reader.File {
		baseName := filepath.Base(file.Name)
		if strings.EqualFold(baseName, targetExe) || strings.EqualFold(baseName, "geoip.dat") || strings.EqualFold(baseName, "geosite.dat") {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			destPath := filepath.Join(targetDir, baseName)
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				rc.Close()
				continue
			}
			_, _ = io.Copy(out, rc)
			out.Close()
			rc.Close()
			_ = os.Chmod(destPath, 0755)
		}
	}
	return nil
}

func extractTarGz(data []byte, targetDir, coreKey string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	targetExe := exeName(coreKey)
	if coreKey == "singbox" {
		targetExe = exeName("sing-box")
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		baseName := filepath.Base(header.Name)
		if strings.EqualFold(baseName, targetExe) {
			destPath := filepath.Join(targetDir, baseName)
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				continue
			}
			_, _ = io.Copy(out, tr)
			out.Close()
			_ = os.Chmod(destPath, 0755)
		}
	}
	return nil
}

func exeName(name string) string {
	if name == "geoip" {
		return "geoip.dat"
	}
	if name == "geosite" {
		return "geosite.dat"
	}
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func dllName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".dll"
	}
	if runtime.GOOS == "darwin" {
		return "lib" + name + ".dylib"
	}
	return "lib" + name + ".so"
}
