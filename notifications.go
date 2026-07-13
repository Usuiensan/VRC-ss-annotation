package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func toastEnabled() bool {
	return appConfig.Notifications.ToastEnabled == nil || *appConfig.Notifications.ToastEnabled
}

func notifyFailure(entry ProcessStateEntry) {
	if !toastEnabled() || runtime.GOOS != "windows" {
		return
	}
	title := "VRC ss annotation"
	message := "処理に失敗しました: " + filepath.Base(entry.SourcePath)
	if entry.Error != "" {
		message += " - " + entry.Error
	}
	script := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; $n=New-Object System.Windows.Forms.NotifyIcon; $n.Icon=[System.Drawing.SystemIcons]::Warning; $n.Visible=$true; $n.ShowBalloonTip(5000,'%s','%s',[System.Windows.Forms.ToolTipIcon]::Warning); Start-Sleep -Seconds 6; $n.Dispose()`,
		escapePowerShellSingleQuoted(title),
		escapePowerShellSingleQuoted(message),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err := cmd.Start(); err != nil {
		appendLog(fmt.Sprintf("notification failed: %v", err))
	}
}

func escapePowerShellSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
