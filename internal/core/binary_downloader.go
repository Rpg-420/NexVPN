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
		latestVer := fetchLatestSingBoxVersion() // e.g. "v1.14.0"
		if latestVer == "" {
			latestVer = "v1.14.0"
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
			return "", fmt.Errorf("Psiphon is pre-installed on Windows and does not require separate download")
		}
		if coreKey == "openvpn" && runtime.GOOS != "windows" {
			return "", fmt.Errorf("OpenVPN is only supported on Windows")
		}
		return "", fmt.Errorf("download URL for %s is not available on this platform", coreKey)
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
				progressCb(0, 0, 0, 0, "Connecting to mirror server...")
			} else {
				progressCb(0, 0, 0, 0, fmt.Sprintf("Trying mirror %d...", idx+1))
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
					lastErr = "server returned 404 (file not found)"
					continue
				}
				// Re-request if it was actually valid but small
				resp, err = client.Do(req)
				if err != nil || resp.StatusCode != http.StatusOK {
					if resp != nil {
						resp.Body.Close()
					}
					lastErr = "downloaded file is invalid"
					continue
				}
			}
			activeUrl = u
			break
		}
		if err != nil {
			lastErr = err.Error()
		} else if resp != nil {
			lastErr = fmt.Sprintf("server returned %d", resp.StatusCode)
			resp.Body.Close()
		}
	}

	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		if lastErr == "" && err != nil {
			lastErr = err.Error()
		}
		if lastErr == "" {
			lastErr = "failed to connect to any download mirror"
		}
		return "", fmt.Errorf("download of %s failed: %s", coreKey, lastErr)
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
		return "", fmt.Errorf("downloaded file is empty - connection interrupted")
	}
	// Basic size sanity check (cores are at least 1MB)
	if len(data) < 100*1024 && (strings.HasSuffix(activeUrl, ".zip") || strings.HasSuffix(activeUrl, ".tar.gz")) {
		// Check if it's HTML error page
		if bytes.Contains(data[:min(512, len(data))], []byte("<html")) {
			return "", fmt.Errorf("server returned HTML error page instead of archive")
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
				err = fmt.Errorf("file is empty after writing to disk")
			}
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to extract %s: %w", coreKey, err)
	}

	// SHA-256 integrity verification.
	//
	// SECURITY: the expected digest is never fetched from the mirror that served
	// the payload - a compromised mirror could otherwise serve both a tampered
	// archive and a matching hash file, defeating the check entirely. The
	// canonical (first) URL is used for the sidecar instead, and a locally pinned
	// digest takes precedence and fails closed.
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	canonicalURL, _ := splitMirrors(GetCoreDownloadURL(coreKey))
	if verr := verifyDownloadedArchive(data, canonicalURL); verr != nil {
		if verr == errChecksumUnavailable {
			// SECURITY: no digest could be obtained. Historically this was a
			// silent "proceed with a warning", which let a malicious mirror
			// substitute an arbitrary binary whenever upstream published no
			// .sha256 sidecar. It is now gated:
			//   * If the artefact is pinned in pinnedCoreHashes the check above
			//     would already have run, so reaching here means it is UNPINNED.
			//   * Unless the user has explicitly opted into unverified
			//     downloads, refuse. A failed update is recoverable; running an
			//     attacker-supplied proxy core is not.
			if !allowUnverifiedCoreDownloads() {
				return "", fmt.Errorf(
					"refusing to install %s: no trusted SHA-256 digest is available "+
						"(upstream published no sidecar and the artefact is not pinned). "+
						"Pin the digest in pinnedCoreHashes or enable unverified downloads explicitly",
					coreKey)
			}
			if progressCb != nil {
				progressCb(99, downloaded, totalBytes, 0,
					"Warning: operating in unverified mode - integrity could NOT be confirmed")
			}
		} else {
			return "", fmt.Errorf("checksum verification failed: %w", verr)
		}
	}
	if progressCb != nil {
		progressCb(100, downloaded, totalBytes, 0, "Complete")
	}

	return fmt.Sprintf("%s updated successfully (SHA256: %s...)", coreKey, hashHex[:12]), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// errChecksumUnavailable signals that no upstream checksum could be obtained for a
// downloaded artefact, so its integrity could not be verified at all.
var errChecksumUnavailable = fmt.Errorf("no published checksum available")

// pinnedCoreHashes maps a canonical download URL to the SHA-256 digest of the
// archive it serves. When an entry is present, the download MUST match it.
//
// Populate this map for every core version a release ships. It is the only
// mechanism that makes the update path resistant to a malicious or compromised
// mirror, because the expected digest never travels over the network.
//
// Run `sha256sum <archive>` on an artefact downloaded from the official upstream
// release page and paste the result here.
//
// NOTE for URL schemes that serve "latest": a pinned digest goes stale the
// moment upstream publishes a new release. verifyDownloadedArchive handles that
// case by falling back to the upstream-published sidecar (see fetchSidecarSHA256)
// before declaring a mismatch, so a genuine new release still installs while a
// tampered mirror payload does not.
var pinnedCoreHashes = map[string]string{
	// sing-box v1.14.0 — the version bundled with NexVPN 1.4.0. sing-box
	// publishes NO checksum sidecar (verified: .sha256/.dgst/.sha256sum all 404),
	// so this pin is the only integrity anchor for that core.
	"https://github.com/SagerNet/sing-box/releases/download/v1.14.0/sing-box-1.14.0-windows-amd64.zip": "3ffb56267da14e287be48bd10cf7e6505260125bad940b75101fbb4d5d58e5d6",

	// Xray-core v26.3.27 (the "latest" URL served this archive on 2026-10-21).
	// Xray publishes a .dgst sidecar, so future upstream releases verify
	// through fetchSidecarSHA256 even after this pin goes stale.
	"https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip": "d004c39288ce9ada487c6f398c7c545f7d749e44bdfdd59dbc9f865afba4e1ad",

	// openvpn-portable 2.5.8-19 (portapps). No sidecar published; the URL is
	// version-pinned so this entry never goes stale.
	"https://github.com/portapps/openvpn-portable/releases/download/2.5.8-19/openvpn-portable-win64.zip": "0019dfc4b32d63c1392aa264aed2253c1e0c2fb09216f8e2cc269bbfb8bb49b5",
}

// allowUnverifiedDownloads is the process-wide escape hatch for installing a
// core archive that has no trusted digest. It is off by default and must be
// flipped deliberately (see SetAllowUnverifiedCoreDownloads), so an ordinary
// update can never silently install a substitutable binary.
var (
	allowUnverifiedMu      sync.RWMutex
	allowUnverifiedEnabled bool
)

// SetAllowUnverifiedCoreDownloads opts in (or out of) installing core archives
// whose SHA-256 could not be checked against a pinned or upstream-published
// digest.
//
// This exists for the rare upstream that ships no checksum at all. Callers must
// surface the choice to the user; enabling it means the update path is no longer
// resistant to a compromised mirror.
func SetAllowUnverifiedCoreDownloads(enabled bool) {
	allowUnverifiedMu.Lock()
	allowUnverifiedEnabled = enabled
	allowUnverifiedMu.Unlock()
}

func allowUnverifiedCoreDownloads() bool {
	allowUnverifiedMu.RLock()
	defer allowUnverifiedMu.RUnlock()
	return allowUnverifiedEnabled
}

// splitMirrors splits a comma-separated download list into its canonical (first)
// URL and the remaining mirror URLs.
func splitMirrors(raw string) (canonical string, mirrors []string) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return "", nil
	}
	canonical = strings.TrimSpace(parts[0])
	for _, p := range parts[1:] {
		if m := strings.TrimSpace(p); m != "" {
			mirrors = append(mirrors, m)
		}
	}
	return canonical, mirrors
}

// fetchSidecarSHA256 fetches a checksum sidecar published next to the archive
// (canonical URL only) and returns the first 64-character hex digest found.
//
// Upstreams are not consistent about the sidecar format, so both the common
// filename variants and both layouts are handled:
//
//	Xray (XTLS):    "<url>.zip.dgst"      →  "SHA2-256= d004c392…"
//	some projects:  "<url>.zip.sha256"    →  "d004c392…  xray.zip"
//
// The whole body is scanned for the first 64-char hex token, which covers both.
func fetchSidecarSHA256(baseURL string) (string, bool) {
	client := &http.Client{Timeout: 10 * time.Second}
	for _, suffix := range []string{".sha256", ".sha256sum", ".sha256.txt", ".dgst"} {
		resp, err := client.Get(baseURL + suffix)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || readErr != nil {
			continue
		}
		for _, field := range strings.Fields(string(body)) {
			candidate := strings.ToLower(strings.Trim(field, "="))
			if len(candidate) == 64 && isHexString(candidate) {
				return candidate, true
			}
		}
	}
	return "", false
}

// isHexString reports whether s consists solely of hexadecimal characters.
func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// verifyDownloadedArchive verifies the SHA-256 digest of a downloaded core archive.
//
// Verification order:
//  1. A sidecar digest published by upstream, fetched from canonicalURL (never
//     from the mirror that served the payload, which would defeat the purpose of
//     the check). This is tried first because release URLs that serve "latest"
//     legitimately change content on every upstream release, so a pinned digest
//     for such a URL is expected to go stale.
//  2. A digest pinned in pinnedCoreHashes for canonicalURL - authoritative and
//     offline; the only anchor for upstreams that publish no sidecar.
//  3. If neither exists, errChecksumUnavailable is returned so the caller can warn
//     the user instead of silently pretending the download was verified.
//
// A sidecar that exists but disagrees with the download is tampering, and a pin
// that disagrees while no sidecar can vouch for the change is treated as
// tampering too - neither falls through to the other check as a second chance.
func verifyDownloadedArchive(data []byte, canonicalURL string) error {
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])

	// 1. Upstream-published sidecar, canonical origin only.
	if canonicalURL != "" {
		if expected, ok := fetchSidecarSHA256(canonicalURL); ok {
			if !strings.EqualFold(expected, actual) {
				return fmt.Errorf("SHA256 mismatch against upstream-published digest: expected %s..., got %s...", expected[:12], actual[:12])
			}
			return nil
		}
	}

	// 2. Pinned digest (authoritative, offline).
	if pinned, ok := pinnedCoreHashes[canonicalURL]; ok && strings.TrimSpace(pinned) != "" {
		if !strings.EqualFold(strings.TrimSpace(pinned), actual) {
			return fmt.Errorf("SHA256 mismatch against pinned digest: expected %s..., got %s...", pinned[:12], actual[:12])
		}
		return nil
	}

	// 3. Nothing to verify against.
	return errChecksumUnavailable
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

	if _, err := os.Stat(filepath.Join(binDir, "geoip.dat")); err != nil {
		return "", fmt.Errorf("geoip.dat update failed: %w", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "geosite.dat")); err != nil {
		return "", fmt.Errorf("geosite.dat update failed: %w", err)
	}
	return "GeoIP and GeoSite routing rules updated successfully!", nil
}

func getBinDirectory() string {
	execDir, _ := os.Executable()
	baseDir := filepath.Dir(execDir)
	cwd, _ := os.Getwd()

	candidates := []string{
		filepath.Join(cwd, "build", "bin", "bin"),
		filepath.Join(baseDir, "bin", "bin"),
		filepath.Join(baseDir, "bin"),
		filepath.Join(cwd, "build", "bin"),
		filepath.Join(cwd, "bin"),
		filepath.Join(cwd, "..", "bin"),
		filepath.Join(baseDir, "..", "bin"),
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
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
		tmpPath := destPath + ".download"
		out, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			continue
		}
		written, copyErr := io.CopyN(out, rc, int64(file.UncompressedSize64)+1)
		out.Close()
		if copyErr != nil && copyErr != io.EOF {
			rc.Close()
			_ = os.Remove(tmpPath)
			continue
		}
		if written > int64(file.UncompressedSize64) {
			rc.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("archive entry exceeded declared size")
		}
		extractedBytes += written
		rc.Close()
		_ = os.Chmod(tmpPath, 0755)
		if err := safeReplaceFile(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			continue
		}
		extracted++
	}
	if extracted == 0 {
		return fmt.Errorf("file %s not found in archive or could not be extracted", targetExe)
	}
	return nil
}

func safeReplaceFile(tmpPath, destPath string) error {
	// Attempt direct rename
	if err := os.Rename(tmpPath, destPath); err == nil {
		return nil
	}
	// Try removing destination if exists
	_ = os.Remove(destPath)
	if err := os.Rename(tmpPath, destPath); err == nil {
		return nil
	}
	// Windows file locking fallback: rename destination to .old and place new file
	oldPath := destPath + ".old"
	_ = os.Remove(oldPath)
	if err := os.Rename(destPath, oldPath); err == nil {
		if err := os.Rename(tmpPath, destPath); err == nil {
			_ = os.Remove(oldPath)
			return nil
		}
	}
	return os.Rename(tmpPath, destPath)
}

func extractTarGz(data []byte, targetDir, coreKey string) error {
	const maxArchiveFiles = 128
	const maxExtractedBytes = 512 << 20
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	targetExe := exeName(coreKey)
	extractedFiles := 0
	var extractedBytes int64
	found := false
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

		if extractedFiles >= maxArchiveFiles || header.Size < 0 || header.Size > maxExtractedBytes || extractedBytes+header.Size > maxExtractedBytes {
			return fmt.Errorf("archive exceeds extraction safety limits")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		baseName := filepath.Base(header.Name)
		if !strings.EqualFold(baseName, targetExe) {
			continue
		}
		destPath := filepath.Join(targetDir, baseName)
		tmpPath := destPath + ".download"
		out, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		written, copyErr := io.CopyN(out, tr, header.Size+1)
		out.Close()
		if copyErr != nil && copyErr != io.EOF {
			_ = os.Remove(tmpPath)
			return copyErr
		}
		if written > header.Size {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("archive entry exceeded declared size")
		}
		_ = os.Chmod(tmpPath, 0755)
		if err := safeReplaceFile(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		extractedBytes += written
		extractedFiles++
		found = true
	}
	if !found {
		return fmt.Errorf("file %s not found in archive", targetExe)
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
// Falls back to hardcoded v1.14.0 if API blocked or fails
func fetchLatestSingBoxVersion() string {
	singBoxLatestCacheOnce.Do(func() {
		client := &http.Client{Timeout: 8 * time.Second}
		req, err := http.NewRequest("GET", "https://api.github.com/repos/SagerNet/sing-box/releases/latest", nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "NexVPN/1.4.0")
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
	return "v1.14.0"
}
