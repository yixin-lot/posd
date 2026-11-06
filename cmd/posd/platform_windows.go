//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// GetConsoleWindow 在 kernel32 里（不是 user32），别搞错。
var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleWin = kernel32.NewProc("GetConsoleWindow")
)

// hasConsole 判断当前进程是否挂在控制台上。
// 桌面版用 -H windowsgui 构建，没有控制台，日志必须落文件。
func hasConsole() bool {
	h, _, _ := procGetConsoleWin.Call()
	return h != 0
}

// openBrowser 用系统默认浏览器打开地址（不弹出黑框）。
func openBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
