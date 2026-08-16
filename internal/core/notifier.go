package core

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"

	"nexvpn/internal/sysproc"
)

// ShowWindowsNotification displays a native system notification across Windows, Linux, and macOS
func ShowWindowsNotification(title, message string) {
	if runtime.GOOS == "windows" {
		titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
		messageB64 := base64.StdEncoding.EncodeToString([]byte(message))

		psScript := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$toastXml = [xml]$template.GetXml()
$title = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$msg = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$toastXml.GetElementsByTagName("text")[0].AppendChild($toastXml.CreateTextNode($title)) > $null
$toastXml.GetElementsByTagName("text")[1].AppendChild($toastXml.CreateTextNode($msg)) > $null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($toastXml.OuterXml)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("NexVPN Client").Show($toast)
`, titleB64, messageB64)

		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		cmd.SysProcAttr = sysproc.HideWindowAttr()
		_ = cmd.Start()
	} else if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		cmd := exec.Command("osascript", "-e", script)
		cmd.SysProcAttr = sysproc.HideWindowAttr()
		_ = cmd.Start()
	} else {
		// Linux notify-send
		cmd := exec.Command("notify-send", title, message, "-a", "NexVPN")
		cmd.SysProcAttr = sysproc.HideWindowAttr()
		_ = cmd.Start()
	}
}
