//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// hasConsole 非 Windows 平台默认有控制台。
func hasConsole() bool { return true }

// openBrowser 在 macOS/Linux 上调用系统打开命令。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
