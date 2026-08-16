package sysproxy

import (
	"os/exec"
	"runtime"
	"strings"

	"nexvpn/internal/sysproc"
)

type InstalledApp struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ExeName   string `json:"exe_name"`
	Category  string `json:"category"` // "browser", "social", "game", "media", "tool", "iranian"
	IsRunning bool   `json:"is_running"`
	IconName  string `json:"icon_name"`
}

var knownAppCatalog = []InstalledApp{
	// Browsers
	{ID: "chrome", Name: "Google Chrome", ExeName: "chrome.exe", Category: "browser", IconName: "Globe"},
	{ID: "firefox", Name: "Mozilla Firefox", ExeName: "firefox.exe", Category: "browser", IconName: "Globe"},
	{ID: "edge", Name: "Microsoft Edge", ExeName: "msedge.exe", Category: "browser", IconName: "Globe"},
	{ID: "brave", Name: "Brave Browser", ExeName: "brave.exe", Category: "browser", IconName: "Globe"},
	{ID: "opera", Name: "Opera / Opera GX", ExeName: "opera.exe", Category: "browser", IconName: "Globe"},

	// Social & Messengers
	{ID: "telegram", Name: "Telegram Desktop", ExeName: "Telegram.exe", Category: "social", IconName: "Send"},
	{ID: "discord", Name: "Discord", ExeName: "Discord.exe", Category: "social", IconName: "MessageSquare"},
	{ID: "whatsapp", Name: "WhatsApp Desktop", ExeName: "WhatsApp.exe", Category: "social", IconName: "MessageCircle"},
	{ID: "slack", Name: "Slack", ExeName: "slack.exe", Category: "social", IconName: "Hash"},

	// Gaming & Launchers
	{ID: "steam", Name: "Steam", ExeName: "steam.exe", Category: "game", IconName: "Gamepad2"},
	{ID: "epic", Name: "Epic Games Launcher", ExeName: "EpicGamesLauncher.exe", Category: "game", IconName: "Gamepad2"},
	{ID: "battlenet", Name: "Battle.net", ExeName: "Battle.net.exe", Category: "game", IconName: "Gamepad2"},
	{ID: "riot", Name: "Riot Client / Valorant", ExeName: "RiotClientServices.exe", Category: "game", IconName: "Gamepad2"},
	{ID: "ea", Name: "EA Desktop", ExeName: "EADesktop.exe", Category: "game", IconName: "Gamepad2"},
	{ID: "roblox", Name: "Roblox", ExeName: "RobloxPlayerBeta.exe", Category: "game", IconName: "Gamepad2"},

	// Media & Streaming
	{ID: "spotify", Name: "Spotify", ExeName: "Spotify.exe", Category: "media", IconName: "Music"},
	{ID: "vlc", Name: "VLC Media Player", ExeName: "vlc.exe", Category: "media", IconName: "Film"},
	{ID: "obs", Name: "OBS Studio", ExeName: "obs64.exe", Category: "media", IconName: "Video"},

	// Tools & Download Managers
	{ID: "idm", Name: "Internet Download Manager (IDM)", ExeName: "IDMan.exe", Category: "tool", IconName: "Download"},
	{ID: "qbittorrent", Name: "qBittorrent", ExeName: "qbittorrent.exe", Category: "tool", IconName: "Download"},
	{ID: "vscode", Name: "Visual Studio Code", ExeName: "Code.exe", Category: "tool", IconName: "Code"},

	// Iranian Domestic Apps (Recommended for Direct Bypass)
	{ID: "rubika", Name: "روبیکا (Rubika)", ExeName: "Rubika.exe", Category: "iranian", IconName: "Shield"},
	{ID: "eitaa", Name: "ایتا (Eitaa)", ExeName: "Eitaa.exe", Category: "iranian", IconName: "Shield"},
	{ID: "bale", Name: "بله (Bale)", ExeName: "Bale.exe", Category: "iranian", IconName: "Shield"},
	{ID: "snapp", Name: "اسنپ (Snapp PWA)", ExeName: "snapp.exe", Category: "iranian", IconName: "Shield"},
	{ID: "digikala", Name: "دیجی کالا (Digikala)", ExeName: "digikala.exe", Category: "iranian", IconName: "Shield"},
}

// GetDetectedApplications returns a catalog of applications with real-time running status
func GetDetectedApplications() []InstalledApp {
	runningMap := make(map[string]bool)

	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
		cmd.SysProcAttr = sysproc.HideWindowAttr()
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				parts := strings.Split(line, ",")
				if len(parts) > 0 {
					exe := strings.Trim(parts[0], `" \r\n`)
					if exe != "" {
						runningMap[strings.ToLower(exe)] = true
					}
				}
			}
		}
	} else {
		cmd := exec.Command("ps", "-A", "-o", "comm=")
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				trimmed := strings.ToLower(strings.TrimSpace(line))
				if trimmed != "" {
					runningMap[trimmed] = true
					runningMap[trimmed+".exe"] = true
				}
			}
		}
	}

	result := make([]InstalledApp, len(knownAppCatalog))
	for i, app := range knownAppCatalog {
		appCopy := app
		if runningMap[strings.ToLower(app.ExeName)] {
			appCopy.IsRunning = true
		}
		result[i] = appCopy
	}

	return result
}
