package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"nexvpn/internal/config"
	"nexvpn/internal/diary"
	"nexvpn/internal/sysproc"
)

// ---------------------------------------------------------------------------
// Launch diary
// ---------------------------------------------------------------------------

// GetDiaryEntries returns the most recent launch-diary entries for the in-app
// timeline.
//
// The diary is the only record of how long each connect phase took, and until
// now the interface could only hand out its file path. Reading it back is what
// turns "connecting feels slow" into "the TUN step took 4.1 s".
func (a *App) GetDiaryEntries(limit int) []diary.Entry {
	if limit <= 0 || limit > 5000 {
		limit = 800
	}
	entries, err := diary.Read(limit)
	if err != nil {
		return []diary.Entry{}
	}
	return entries
}

// ---------------------------------------------------------------------------
// Connection statistics
// ---------------------------------------------------------------------------

// DailyUsage is one day of aggregated session data.
type DailyUsage struct {
	Date        string `json:"date"`
	Sessions    int    `json:"sessions"`
	DurationSec int64  `json:"duration_sec"`
	BytesRx     int64  `json:"bytes_rx"`
	BytesTx     int64  `json:"bytes_tx"`
}

// ServerUsage is one server's share of the recorded history.
type ServerUsage struct {
	ProfileName string `json:"profile_name"`
	Protocol    string `json:"protocol"`
	Sessions    int    `json:"sessions"`
	DurationSec int64  `json:"duration_sec"`
	Bytes       int64  `json:"bytes"`
}

// ConnectionStatistics summarises the recorded connection history.
type ConnectionStatistics struct {
	TotalSessions      int           `json:"total_sessions"`
	TotalDurationSec   int64         `json:"total_duration_sec"`
	TotalBytesRx       int64         `json:"total_bytes_rx"`
	TotalBytesTx       int64         `json:"total_bytes_tx"`
	AverageDurationSec int64         `json:"average_duration_sec"`
	WindowDays         int           `json:"window_days"`
	UptimePercent      float64       `json:"uptime_percent"`
	Daily              []DailyUsage  `json:"daily"`
	TopServers         []ServerUsage `json:"top_servers"`
}

// statisticsWindowDays bounds every figure in ConnectionStatistics.
//
// A window rather than "all time" because the useful question is "how has this
// week been", and because history.json grows without bound — an all-time
// average is dominated by whatever happened months ago and stops responding to
// anything the user does now.
const statisticsWindowDays = 30

// GetConnectionStatistics aggregates history.json into the figures the
// statistics view shows.
func (a *App) GetConnectionStatistics() ConnectionStatistics {
	history := a.store.GetConnectionHistory()

	since := time.Now().AddDate(0, 0, -statisticsWindowDays)
	stats := ConnectionStatistics{
		WindowDays: statisticsWindowDays,
		Daily:      []DailyUsage{},
		TopServers: []ServerUsage{},
	}

	byDay := map[string]*DailyUsage{}
	byServer := map[string]*ServerUsage{}

	var earliest time.Time
	for _, session := range history {
		if session.ConnectedAt.Before(since) {
			continue
		}

		stats.TotalSessions++
		stats.TotalDurationSec += session.DurationSec
		stats.TotalBytesRx += session.BytesReceived
		stats.TotalBytesTx += session.BytesSent

		if earliest.IsZero() || session.ConnectedAt.Before(earliest) {
			earliest = session.ConnectedAt
		}

		day := session.ConnectedAt.Format("2006-01-02")
		if byDay[day] == nil {
			byDay[day] = &DailyUsage{Date: day}
		}
		byDay[day].Sessions++
		byDay[day].DurationSec += session.DurationSec
		byDay[day].BytesRx += session.BytesReceived
		byDay[day].BytesTx += session.BytesSent

		// A profile can be renamed, so the key is the name rather than the id:
		// the user recognises the name they see in the list today.
		key := session.ProfileName
		if key == "" {
			key = "unknown"
		}
		if byServer[key] == nil {
			byServer[key] = &ServerUsage{ProfileName: key, Protocol: string(session.Protocol)}
		}
		byServer[key].Sessions++
		byServer[key].DurationSec += session.DurationSec
		byServer[key].Bytes += session.BytesReceived + session.BytesSent
	}

	if stats.TotalSessions > 0 {
		stats.AverageDurationSec = stats.TotalDurationSec / int64(stats.TotalSessions)
	}

	// Uptime is measured from the first recorded session inside the window, not
	// from "30 days ago": on a freshly installed app the latter would report
	// almost 0% and look like a fault.
	if !earliest.IsZero() {
		window := time.Since(earliest).Seconds()
		if window > 0 {
			stats.UptimePercent = float64(stats.TotalDurationSec) / window * 100
			if stats.UptimePercent > 100 {
				stats.UptimePercent = 100
			}
		}
	}

	for _, day := range byDay {
		stats.Daily = append(stats.Daily, *day)
	}
	sort.Slice(stats.Daily, func(i, j int) bool { return stats.Daily[i].Date < stats.Daily[j].Date })

	for _, server := range byServer {
		stats.TopServers = append(stats.TopServers, *server)
	}
	sort.Slice(stats.TopServers, func(i, j int) bool {
		return stats.TopServers[i].DurationSec > stats.TopServers[j].DurationSec
	})
	if len(stats.TopServers) > 8 {
		stats.TopServers = stats.TopServers[:8]
	}

	return stats
}

// ---------------------------------------------------------------------------
// IPv6 leak test
// ---------------------------------------------------------------------------

// IPv6LeakResult reports whether IPv6 traffic is escaping the tunnel.
type IPv6LeakResult struct {
	TunnelActive bool   `json:"tunnel_active"`
	IPv6Reachable bool  `json:"ipv6_reachable"`
	Leaking      bool   `json:"leaking"`
	ObservedIP   string `json:"observed_ip"`
	Detail       string `json:"detail"`
}

// ipv6ProbeURL is an IPv6-only endpoint. It is used rather than a dual-stack
// one because the question is not "does IPv6 work" but "does IPv6 traffic leave
// this machine without going through the tunnel".
const ipv6ProbeURL = "https://ipv6.icanhazip.com"

// TestIPv6Leak checks whether IPv6 traffic bypasses the tunnel.
//
// Why this matters more than it sounds: IPv4-only tunnel rules are the norm,
// and a machine with working IPv6 will happily send that traffic out of the
// physical interface while the IPv4 tunnel looks perfectly healthy. The leak is
// invisible in the app and invisible to the user, which is exactly the kind of
// leak a VPN client should be able to prove it is not causing.
//
// The check is only meaningful while connected, so it reports that state rather
// than guessing.
func (a *App) TestIPv6Leak() IPv6LeakResult {
	result := IPv6LeakResult{
		TunnelActive: a.GetConnectionStatus().State == "connected",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			// Dial explicitly over tcp6: with a dual-stack dialer Go may fall
			// back to IPv4 and report a false "no leak".
			DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
				_, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				dialer := &net.Dialer{Timeout: 6 * time.Second}
				return dialer.DialContext(ctx, "tcp6", net.JoinHostPort("ipv6.icanhazip.com", port))
			},
			TLSHandshakeTimeout: 6 * time.Second,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ipv6ProbeURL, nil)
	if err != nil {
		result.Detail = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "NexVPN/1.3")

	resp, err := client.Do(req)
	if err != nil {
		// No IPv6 route at all, or IPv6 is correctly blocked. Either way nothing
		// is leaking, so this is a pass — but the wording distinguishes them so
		// the user is not told "protected" when the truth is "not applicable".
		result.IPv6Reachable = false
		if result.TunnelActive {
			result.Detail = "IPv6 traffic could not leave the machine — nothing is leaking"
		} else {
			result.Detail = "No IPv6 route is available on this machine, so there is nothing to leak"
		}
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
	address := strings.TrimSpace(string(body))
	result.IPv6Reachable = true
	result.ObservedIP = address

	if result.TunnelActive {
		result.Leaking = true
		result.Detail = fmt.Sprintf(
			"IPv6 traffic reached the internet directly (observed %s) while the tunnel was up. "+
				"Set the IP strategy to IPv4-only, or choose a core that routes IPv6, to close this leak.",
			address)
		return result
	}

	result.Detail = fmt.Sprintf("IPv6 is reachable (observed %s), but no tunnel is active", address)
	return result
}

// ---------------------------------------------------------------------------
// Application update check
// ---------------------------------------------------------------------------

// UpdateInfo describes the result of an update check.
type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url"`
	ReleaseNotes    string `json:"release_notes"`
	PublishedAt     string `json:"published_at"`
	CheckedAt       string `json:"checked_at"`
	Detail          string `json:"detail"`
}

// releaseAPIURL is the GitHub releases endpoint for this project.
//
// Only the app itself is checked here. The proxy cores have their own
// download-and-verify path (binary_downloader.go) and are updated separately.
const releaseAPIURL = "https://api.github.com/repos/Rpg-420/NexVPN/releases/latest"

// CheckAppUpdate compares the running version against the latest published
// release.
//
// It never downloads or installs anything: the check is a single unauthenticated
// GET, and the result is a link the user chooses to follow. An app that silently
// replaces its own binary is a much larger trust decision than one that tells
// you a newer version exists.
func (a *App) CheckAppUpdate() UpdateInfo {
	info := UpdateInfo{
		CurrentVersion: AppVersion,
		CheckedAt:      time.Now().Format(time.RFC3339),
	}

	settings := a.store.GetSettings()
	if !settings.AppUpdateCheckEnabled {
		info.Detail = "Update checks are turned off in Settings"
		return info
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		info.Detail = err.Error()
		return info
	}
	// GitHub rejects requests without a User-Agent, and an unauthenticated
	// request is rate limited to 60/hour per address — ample for a manual check.
	req.Header.Set("User-Agent", "NexVPN/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		info.Detail = "Could not reach the release server: " + err.Error()
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.Detail = fmt.Sprintf("The release server answered HTTP %d", resp.StatusCode)
		return info
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		info.Detail = "The release server sent something that could not be read"
		return info
	}
	if payload.Draft || payload.Prerelease {
		info.Detail = "The latest published release is a draft or pre-release"
		return info
	}

	info.LatestVersion = strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	info.ReleaseURL = payload.HTMLURL
	info.ReleaseNotes = payload.Body
	info.PublishedAt = payload.PublishedAt
	info.UpdateAvailable = compareVersions(info.LatestVersion, AppVersion) > 0

	if info.UpdateAvailable {
		info.Detail = fmt.Sprintf("Version %s is available", info.LatestVersion)
	} else {
		info.Detail = "You are running the latest version"
	}
	return info
}

// compareVersions compares two dotted numeric versions, returning a positive
// number when a is newer than b.
//
// It is deliberately not a semver library: the app's versions are plain
// `major.minor.patch` with no pre-release identifiers, and pulling in a full
// parser to compare three integers would be more surface area than the problem
// has. A non-numeric component ends the comparison, which makes a malformed tag
// compare as "not newer" rather than as an error the user cannot act on.
func compareVersions(a, b string) int {
	parse := func(v string) []int {
		parts := strings.Split(strings.TrimSpace(v), ".")
		out := make([]int, 0, len(parts))
		for _, part := range parts {
			// Trim anything after a non-numeric character so "5-beta" yields 5.
			numeric := part
			for i, r := range part {
				if r < '0' || r > '9' {
					numeric = part[:i]
					break
				}
			}
			n, err := strconv.Atoi(numeric)
			if err != nil {
				break
			}
			out = append(out, n)
		}
		return out
	}

	av, bv := parse(a), parse(b)
	for i := 0; i < len(av) || i < len(bv); i++ {
		var ai, bi int
		if i < len(av) {
			ai = av[i]
		}
		if i < len(bv) {
			bi = bv[i]
		}
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Config rollback
// ---------------------------------------------------------------------------

// RestorePreviousConfig rolls the configuration back to the pre-write snapshot.
func (a *App) RestorePreviousConfig() error {
	if err := a.store.RestorePreviousConfig(); err != nil {
		return err
	}
	a.engine.AddLog("warning", "Configuration rolled back to the previous snapshot", "app")
	diary.Log("config", "rollback-ok")
	return nil
}

// HasPreviousConfigSnapshot reports whether a rollback is possible, so the
// interface can disable the button instead of offering an action that fails.
func (a *App) HasPreviousConfigSnapshot() bool {
	info, err := os.Stat(a.store.PreviousConfigSnapshotPath())
	return err == nil && info.Size() > 0
}

// GetSubscriptionChanges returns what recent subscription refreshes changed.
func (a *App) GetSubscriptionChanges() []config.SubscriptionChange {
	changes := a.store.GetSubscriptionChanges()
	// Most recent first: the notice the user wants is about the refresh that
	// just happened, not the one from last week.
	for i, j := 0, len(changes)-1; i < j; i, j = i+1, j-1 {
		changes[i], changes[j] = changes[j], changes[i]
	}
	return changes
}

// ClearSubscriptionChanges dismisses the recorded refresh notices.
func (a *App) ClearSubscriptionChanges() {
	a.store.ClearSubscriptionChanges()
}

// ---------------------------------------------------------------------------
// Diagnostic bundle
// ---------------------------------------------------------------------------

// ExportDiagnosticsBundle writes a zip of everything needed to diagnose a
// problem, and returns the path it was written to (empty when the user cancels
// the save dialog).
//
// Secrets are removed rather than obfuscated. A bundle is made to be sent to
// someone else, and "redacted" has to mean the credentials are not in the file
// at all — a bundle that leaks a user's server credentials to whoever they ask
// for help is a worse outcome than a slightly less complete bug report.
func (a *App) ExportDiagnosticsBundle() (string, error) {
	ctx := a.appCtx()
	if ctx == nil {
		return "", fmt.Errorf("the application is not ready yet")
	}

	target, err := wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save NexVPN diagnostics",
		DefaultFilename: fmt.Sprintf("nexvpn-diagnostics-%s.zip", time.Now().Format("20060102-150405")),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Zip archive (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", err
	}
	if target == "" {
		return "", nil // the user cancelled; not an error
	}

	file, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("could not create %s: %w", target, err)
	}

	archive := zip.NewWriter(file)

	addFile := func(name string, write func(w io.Writer) error) {
		entry, err := archive.Create(name)
		if err != nil {
			return
		}
		_ = write(entry)
	}

	addFile("system-info.txt", func(w io.Writer) error {
		_, err := fmt.Fprintf(w, strings.Join([]string{
			"NexVPN diagnostics bundle",
			"generated: %s",
			"version: %s",
			"os: %s/%s",
			"go: %s",
			"admin: %t",
			"diary: %s",
			"data dir: %s",
			"",
		}, "\n"),
			time.Now().Format(time.RFC3339),
			AppVersion,
			runtime.GOOS, runtime.GOARCH,
			runtime.Version(),
			sysproc.IsAdmin(),
			diary.Path(),
			a.store.GetDataDir(),
		)
		return err
	})

	addFile("engine-log.txt", func(w io.Writer) error {
		for _, entry := range a.engine.GetLogs() {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				entry.Timestamp.Format(time.RFC3339), entry.Level, entry.Source, entry.Message); err != nil {
				return err
			}
		}
		return nil
	})

	// The diary is copied verbatim: it is the most valuable file in the bundle
	// and it contains no credentials — only timings, states and profile names.
	addFile("startup-diary.log", func(w io.Writer) error {
		source, err := os.Open(diary.Path())
		if err != nil {
			return err
		}
		defer source.Close()
		_, err = io.Copy(w, io.LimitReader(source, 4<<20))
		return err
	})

	addFile("config.redacted.json", func(w io.Writer) error {
		redacted := redactSettingsForDiagnostics(a.store.GetSettings())
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(redacted)
	})

	addFile("profiles.redacted.json", func(w io.Writer) error {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(redactProfilesForDiagnostics(a.store.GetProfiles()))
	})

	addFile("connection-history.json", func(w io.Writer) error {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(a.store.GetConnectionHistory())
	})

	if err := archive.Close(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}

	diary.Log("diagnostics", "bundle-exported", "path", filepath.Base(target))
	a.engine.AddLog("info", fmt.Sprintf("Diagnostics bundle written to %s", target), "app")
	return target, nil
}

// redactedPlaceholder replaces every secret in a diagnostics bundle.
const redactedPlaceholder = "<redacted>"

// redactSettingsForDiagnostics removes settings that are secrets.
func redactSettingsForDiagnostics(settings config.Settings) config.Settings {
	if settings.WarpLicenseKey != "" {
		settings.WarpLicenseKey = redactedPlaceholder
	}
	if settings.QuotaWebhookURL != "" {
		settings.QuotaWebhookURL = redactedPlaceholder
	}
	if settings.CustomDoHUrl != "" {
		settings.CustomDoHUrl = redactedPlaceholder
	}
	return settings
}

// redactProfilesForDiagnostics strips credentials and config bodies from every
// profile, keeping the shape and the non-sensitive metadata.
//
// The bodies are removed entirely rather than replaced: an OpenVPN config or a
// VLESS URI *is* a credential, and there is no reason to ship even a marker for
// it when the metadata already says which protocol and server were involved.
func redactProfilesForDiagnostics(profiles []config.Profile) []config.Profile {
	out := make([]config.Profile, 0, len(profiles))
	for _, profile := range profiles {
		profile.ConfigContent = ""
		profile.RawURI = ""
		profile.Password = ""
		if profile.Username != "" {
			profile.Username = redactedPlaceholder
		}
		out = append(out, profile)
	}
	return out
}
