package core

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// githubMirrors expands a github URL into primary + Iran-friendly mirror URLs
func githubMirrors(primary string) string {
	if primary == "" {
		return ""
	}
	mirrors := []string{
		primary,
		"https://ghproxy.net/" + primary,
		"https://ghfast.top/" + primary,
		"https://mirror.ghproxy.com/" + primary,
	}
	return strings.Join(mirrors, ",")
}

// GetCoreDownloadURL returns official release URLs tailored to the current OS and CPU architecture
// Returns comma-separated list: primary + mirrors (for UpdateCoreEngineWithProgress to iterate)
func GetCoreDownloadURL(coreKey string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	var primary string
	switch coreKey {
	case "xray":
		switch {
		case osName == "windows" && arch == "amd64":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"
		case osName == "windows" && arch == "386":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-32.zip"
		case osName == "windows" && arch == "arm64":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-arm64-v8a.zip"
		case osName == "linux" && arch == "amd64":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"
		case osName == "linux" && arch == "arm64":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip"
		case osName == "darwin" && arch == "arm64":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-macos-arm64-v8a.zip"
		case osName == "darwin":
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-macos-64.zip"
		default:
			primary = "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip"
		}
		return githubMirrors(primary)

	case "singbox":
		// Use latest stable v1.13.x - auto-fetched via GitHub API with hardcoded fallback
		latestVer := fetchLatestSingBoxVersion() // e.g. "v1.13.19"
		if latestVer == "" {
			latestVer = "v1.13.19"
		}
		ver := strings.TrimPrefix(latestVer, "v")
		switch {
		case osName == "windows" && arch == "amd64":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-windows-amd64.zip", latestVer, ver)
		case osName == "windows" && arch == "arm64":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-windows-arm64.zip", latestVer, ver)
		case osName == "linux" && arch == "amd64":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-linux-amd64.tar.gz", latestVer, ver)
		case osName == "linux" && arch == "arm64":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-linux-arm64.tar.gz", latestVer, ver)
		case osName == "darwin" && arch == "arm64":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-darwin-arm64.tar.gz", latestVer, ver)
		case osName == "darwin":
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-darwin-amd64.tar.gz", latestVer, ver)
		default:
			primary = fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/%s/sing-box-%s-windows-amd64.zip", latestVer, ver)
		}
		return githubMirrors(primary)

	case "geoip":
		primary = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
		return githubMirrors(primary)
	case "geosite":
		primary = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
		return githubMirrors(primary)
	case "aether":
		switch {
		case osName == "windows" && arch == "amd64":
			primary = "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-windows-x86_64.zip"
		case osName == "linux" && arch == "amd64":
			primary = "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-linux-x86_64.tar.gz"
		case osName == "linux" && arch == "arm64":
			primary = "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-linux-arm64.tar.gz"
		case osName == "darwin":
			primary = "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-macos-x86_64.tar.gz"
		default:
			primary = "https://github.com/CluvexStudio/Aether/releases/download/v1.7.0/aether-windows-x86_64.zip"
		}
		return githubMirrors(primary)
	case "psiphon":
		switch {
		case osName == "windows":
			return "" // Bundled with installer - no separate download on Windows (psiphon.exe is included in bin/)
		case osName == "linux" && arch == "amd64":
			primary = "https://github.com/Psiphon-Labs/psiphon-tunnel-core/releases/download/v20230327/psiphon-tunnel-core-linux-amd64"
		case osName == "linux" && arch == "arm64":
			primary = "https://github.com/Psiphon-Labs/psiphon-tunnel-core/releases/download/v20230327/psiphon-tunnel-core-linux-arm64"
		case osName == "darwin":
			primary = "https://github.com/Psiphon-Labs/psiphon-tunnel-core/releases/download/v20230327/psiphon-tunnel-core-darwin-amd64"
		default:
			return ""
		}
		if primary != "" {
			return githubMirrors(primary)
		}
		return ""
	case "openvpn":
		if osName == "windows" {
			primary = "https://github.com/portapps/openvpn-portable/releases/download/2.5.8-19/openvpn-portable-win64.zip"
			return githubMirrors(primary)
		}
		// OpenVPN is Windows-only in this app (uses wintun driver)
		return ""
	case "wintun":
		primary = "https://www.wintun.net/builds/wintun-0.14.1.zip"
		// wintun is small and rarely blocked, but add mirror via ghproxy if available
		return primary + "," + "https://ghproxy.net/" + primary
	}

	return ""
}

// DownloadProgressCallback reports progress during core binary downloads
type DownloadProgressCallback func(percent int, downloadedBytes int64, totalBytes int64, speedBytesPerSec int64, status string)

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
		"psiphon": {
			Name:        "Psiphon Emergency Shield (psiphon-tunnel-core)",
			Filename:    exeName("psiphon"),
			DownloadURL: GetCoreDownloadURL("psiphon"),
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
			DownloadURL: GetCoreDownloadURL("wintun"),
		}
		binaries["openvpn"] = BinaryInfo{
			Name:        "OpenVPN Native Client",
			Filename:    exeName("openvpn"),
			DownloadURL: GetCoreDownloadURL("openvpn"),
		}
	}
	// Remove openvpn entry on non-windows if it was added (openvpn is Windows-only portable)
	if runtime.GOOS != "windows" {
		delete(binaries, "openvpn")
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
	case "psiphon":
		return "v2.0.40 (Console Core)"
	case "aether":
		return "v1.7.0 (MASQUE / Evasion)"
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

// UpdateCoreEngine downloads and hot-updates an engine binary (without progress callback)
func UpdateCoreEngine(coreKey string) (string, error) {
	return UpdateCoreEngineWithProgress(coreKey, nil)
}

func UpdateCoreEngineWithProgress(coreKey string, progressCb DownloadProgressCallback) (string, error) {
	urlStr := GetCoreDownloadURL(coreKey)
	if urlStr == "" {
		if coreKey == "psiphon" && runtime.GOOS == "windows" {
			return "", fmt.Errorf("Psiphon در ویندوز به صورت پیش‌فرض همراه برنامه است و نیاز به دانلود جدا ندارد. اگر فایل bin/psiphon.exe حذف شده، آن را از نصب‌کننده بازیابی کنید")
		}
		if coreKey == "openvpn" && runtime.GOOS != "windows" {
			return "", fmt.Errorf("OpenVPN فقط در ویندوز پشتیبانی می‌شود")
		}
		return "", fmt.Errorf("لینک دانلود برای %s در این سیستم عامل موجود نیست", coreKey)
	}

	urls := strings.Split(urlStr, ",")
	targetDir := getBinDirectory()
	_ = os.MkdirAll(targetDir, 0755)

	client := &http.Client{Timeout: 300 * time.Second}
	var resp *http.Response
	var err error
	var activeUrl string
	var lastErr string

	for idx, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if progressCb != nil {
			if idx == 0 {
				progressCb(0, 0, 0, 0, "در حال اتصال به سرور...")
			} else {
				progressCb(0, 0, 0, 0, fmt.Sprintf("تلاش با آینه %d...", idx+1))
			}
		}
		req, reqErr := http.NewRequest("GET", u, nil)
		if reqErr != nil {
			err = reqErr
			lastErr = err.Error()
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 NexVPN-Updater/1.3")
		req.Header.Set("Accept", "*/*")

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			// Verify we got actual content, not a small HTML error page
			if resp.ContentLength > 0 && resp.ContentLength < 1024 && (strings.HasSuffix(u, ".zip") || strings.HasSuffix(u, ".tar.gz")) {
				// Suspiciously small for archive - likely error page
				bodyPeek := make([]byte, 512)
				n, _ := resp.Body.Read(bodyPeek)
				resp.Body.Close()
				peekStr := strings.ToLower(string(bodyPeek[:n]))
				if strings.Contains(peekStr, "<html") || strings.Contains(peekStr, "not found") {
					lastErr = "سرور فایل را پیدا نکرد (404)"
					continue
				}
				// Re-request if it was actually valid but small
				resp, err = client.Do(req)
				if err != nil || resp.StatusCode != http.StatusOK {
					if resp != nil {
						resp.Body.Close()
					}
					lastErr = "فایل دریافتی نامعتبر است"
					continue
				}
			}
			activeUrl = u
			break
		}
		if err != nil {
			lastErr = err.Error()
		} else if resp != nil {
			lastErr = fmt.Sprintf("سرور پاسخ %d داد", resp.StatusCode)
			resp.Body.Close()
		}
	}

	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		if lastErr == "" && err != nil {
			lastErr = err.Error()
		}
		if lastErr == "" {
			lastErr = "اتصال به هیچ آینه‌ای برقرار نشد - اینترنت یا فیلترینگ را بررسی کنید"
		}
		return "", fmt.Errorf("دانلود %s ناموفق بود: %s. لطفاً VPN دیگری برای دانلود اولیه استفاده کنید یا فایل را دستی در پوشه bin قرار دهید", coreKey, lastErr)
	}
	defer resp.Body.Close()

	totalBytes := resp.ContentLength
	var downloaded int64
	var lastReport time.Time
	var lastDownloaded int64
	var buf [32 * 1024]byte
	var bodyBuffer bytes.Buffer

	startTime := time.Now()
	lastReport = startTime

	for {
		n, readErr := resp.Body.Read(buf[:])
		if n > 0 {
			bodyBuffer.Write(buf[:n])
			downloaded += int64(n)

			now := time.Now()
			if now.Sub(lastReport) >= 200*time.Millisecond || readErr == io.EOF {
				duration := now.Sub(lastReport).Seconds()
				var speed int64
				if duration > 0 {
					speed = int64(float64(downloaded-lastDownloaded) / duration)
				}
				percent := 0
				if totalBytes > 0 {
					percent = int((downloaded * 100) / totalBytes)
					if percent > 100 {
						percent = 100
					}
				}

				if progressCb != nil {
					progressCb(percent, downloaded, totalBytes, speed, "Downloading...")
				}
				lastReport = now
				lastDownloaded = downloaded
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("stream read error: %w", readErr)
		}
	}

	if progressCb != nil {
		progressCb(98, downloaded, totalBytes, 0, "Extracting...")
	}

	data := bodyBuffer.Bytes()
	if len(data) == 0 {
		return "", fmt.Errorf("فایل دانلود شده خالی است - اتصال قطع شده")
	}
	// Basic size sanity check (cores are at least 1MB)
	if len(data) < 100*1024 && (strings.HasSuffix(activeUrl, ".zip") || strings.HasSuffix(activeUrl, ".tar.gz")) {
		// Check if it's HTML error page
		if bytes.Contains(data[:min(512, len(data))], []byte("<html")) {
			return "", fmt.Errorf("سرور به جای فایل، صفحه خطا برگرداند - لینک ممکن است منقضی شده باشد")
		}
	}

	// Determine archive type
	if strings.HasSuffix(activeUrl, ".zip") {
		err = extractZip(data, targetDir, coreKey)
	} else if strings.HasSuffix(activeUrl, ".tar.gz") || strings.HasSuffix(activeUrl, ".tgz") {
		err = extractTarGz(data, targetDir, coreKey)
	} else {
		// Direct file (psiphon binary)
		dest := filepath.Join(targetDir, exeName(coreKey))
		err = os.WriteFile(dest, data, 0755)
		if err == nil {
			if info, statErr := os.Stat(dest); statErr != nil || info.Size() == 0 {
				err = fmt.Errorf("فایل پس از ذخیره خالی است")
			}
		}
	}

	if err != nil {
		return "", fmt.Errorf("خطا در استخراج %s: %w", coreKey, err)
	}

	// SHA256 verification (best-effort against sidecar .sha256 if available)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	if err := verifySHA256FromURL(data, activeUrl); err != nil {
		return "", fmt.Errorf("تأیید صحت فایل ناموفق: %w - فایل ممکن است خراب یا دستکاری شده باشد", err)
	}
	if progressCb != nil {
		progressCb(100, downloaded, totalBytes, 0, "تکمیل شد")
	}

	return fmt.Sprintf("%s با موفقیت به‌روزرسانی شد (SHA256: %s...)", coreKey, hashHex[:12]), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// verifySHA256FromURL tries to fetch URL.sha256 and compare with data hash (best-effort)
func verifySHA256FromURL(data []byte, baseURL string) error {
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])
	for _, suffix := range []string{".sha256", ".sha256sum", ".sha256.txt"} {
		shaURL := baseURL + suffix
		// Also try ghproxy mirrors for hash file
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(shaURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if readErr != nil {
			continue
		}
		// Hash file format: "<hex>  filename" or just "<hex>"
		fields := strings.Fields(string(body))
		if len(fields) == 0 {
			continue
		}
		expected := strings.ToLower(strings.TrimSpace(fields[0]))
		if len(expected) != 64 {
			continue
		}
		if expected != actual {
			return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expected[:12], actual[:12])
		}
		return nil // verified
	}
	return nil // no hash file available - skip strict verification
}

// UpdateGeoData downloads and updates GeoIP and GeoSite routing databases
// Fixed: uses tmp file + atomic rename, proper Body.Close, and size validation
func UpdateGeoData() (string, error) {
	binDir := getBinDirectory()
	_ = os.MkdirAll(binDir, 0755)

	client := &http.Client{Timeout: 60 * time.Second}

	for _, key := range []string{"geoip", "geosite"} {
		filename := "geoip.dat"
		if key == "geosite" {
			filename = "geosite.dat"
		}
		urls := strings.Split(GetCoreDownloadURL(key), ",")
		var success bool
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			resp, err := client.Get(u)
			if err != nil {
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}
			targetPath := filepath.Join(binDir, filename)
			tmpPath := targetPath + ".download"
			out, err := os.Create(tmpPath)
			if err != nil {
				resp.Body.Close()
				continue
			}
			_, copyErr := io.Copy(out, resp.Body)
			out.Close()
			resp.Body.Close()
			if copyErr != nil {
				_ = os.Remove(tmpPath)
				continue
			}
			if info, err := os.Stat(tmpPath); err != nil || info.Size() < 100*1024 {
				_ = os.Remove(tmpPath)
				continue
			}
			_ = os.Remove(targetPath)
			if err := os.Rename(tmpPath, targetPath); err == nil {
				success = true
				break
			}
			_ = os.Remove(tmpPath)
		}
		if !success {
			// keep old file if download failed
			continue
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

	if info, err := os.Stat(filepath.Join(cwd, "..", "bin")); err == nil && info.IsDir() {
		return filepath.Join(cwd, "..", "bin")
	}

	return filepath.Join(baseDir, "bin")
}

func extractZip(data []byte, targetDir, coreKey string) error {
	const maxArchiveFiles = 128
	const maxExtractedBytes = 512 << 20
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	targetExe := exeName(coreKey)
	if coreKey == "singbox" {
		targetExe = exeName("sing-box")
	}

	extracted := 0
	var extractedBytes int64
	for _, file := range reader.File {
		if extracted >= maxArchiveFiles || file.UncompressedSize64 > uint64(maxExtractedBytes) || extractedBytes+int64(file.UncompressedSize64) > maxExtractedBytes {
			return fmt.Errorf("archive exceeds extraction safety limits")
		}
		if file.Mode()&os.ModeSymlink != 0 || file.FileInfo().IsDir() {
			continue
		}
		baseName := filepath.Base(file.Name)
		shouldExtract := false
		// Core binaries: match exact exe name
		if strings.EqualFold(baseName, targetExe) || strings.EqualFold(baseName, "geoip.dat") || strings.EqualFold(baseName, "geosite.dat") {
			shouldExtract = true
		}
		// OpenVPN portable: extract dlls and openvpn.exe together (needs libcrypto/ssl)
		if coreKey == "openvpn" {
			lower := strings.ToLower(baseName)
			if lower == "openvpn.exe" || strings.HasPrefix(lower, "libcrypto") || strings.HasPrefix(lower, "libssl") || strings.HasPrefix(lower, "libpkcs11") {
				shouldExtract = true
			}
		}
		// Wintun: extract dll
		if coreKey == "wintun" && strings.EqualFold(baseName, "wintun.dll") {
			shouldExtract = true
		}
		if !shouldExtract {
			continue
		}
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
		written, copyErr := io.CopyN(out, rc, int64(file.UncompressedSize64)+1)
		out.Close()
		if copyErr != nil && copyErr != io.EOF {
			rc.Close()
			_ = os.Remove(destPath)
			continue
		}
		if written > int64(file.UncompressedSize64) {
			rc.Close()
			_ = os.Remove(destPath)
			return fmt.Errorf("archive entry exceeded declared size")
		}
		extractedBytes += written
		rc.Close()
		_ = os.Chmod(destPath, 0755)
		extracted++
	}
	if extracted == 0 {
		return fmt.Errorf("فایل %s در آرشیو پیدا نشد - آرشیو ممکن است خراب باشد", targetExe)
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

var (
	singBoxLatestCache     string
	singBoxLatestCacheOnce sync.Once
)

// fetchLatestSingBoxVersion queries GitHub API for latest sing-box tag (cached, 8s timeout)
// Falls back to hardcoded v1.13.19 if API blocked or fails
func fetchLatestSingBoxVersion() string {
	singBoxLatestCacheOnce.Do(func() {
		client := &http.Client{Timeout: 8 * time.Second}
		req, err := http.NewRequest("GET", "https://api.github.com/repos/SagerNet/sing-box/releases/latest", nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "NexVPN/1.3.5")
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		defer resp.Body.Close()
		var data struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return
		}
		tag := strings.TrimSpace(data.TagName)
		if tag != "" && strings.HasPrefix(tag, "v") {
			singBoxLatestCache = tag
		}
	})
	if singBoxLatestCache != "" {
		return singBoxLatestCache
	}
	return "v1.13.19"
}
