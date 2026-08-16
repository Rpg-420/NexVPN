package core

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

// ShowWindowsNotification displays a native Windows system notification
func ShowWindowsNotification(title, message string) {
	if runtime.GOOS != "windows" {
		return
	}

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
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = cmd.Start()
}
