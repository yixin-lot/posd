//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var appBrowser *exec.Cmd

// openAppWindow 用默认浏览器的独立配置打开一个 App 窗口。
// 独立 user-data-dir 保证：不碰用户已有的浏览器配置/窗口，退出时也能只关自己。
func openAppWindow(url string) (*exec.Cmd, error) {
	exe, ok := defaultBrowserExe()
	if !ok {
		return nil, fmt.Errorf("未识别默认浏览器")
	}
	profile := filepath.Join(os.Getenv("LOCALAPPDATA"), "Posd", "browser")
	_ = os.MkdirAll(profile, 0o755)
	cmd := exec.Command(exe,
		"--app="+url,
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// closeAppWindow 只结束我们自己启动的那个进程树；不会动其他浏览器窗口。
func closeAppWindow() {
	if appBrowser == nil || appBrowser.Process == nil {
		return
	}
	pid := appBrowser.Process.Pid
	_ = appBrowser.Process.Kill()
	kill := exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F")
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
	appBrowser = nil
}

// defaultBrowserExe 读注册表拿默认浏览器 ProgId，再映射到常见可执行文件。
func defaultBrowserExe() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	progID, _, err := k.GetStringValue("ProgId")
	if err != nil {
		return "", false
	}
	candidates := map[string][]string{
		"MSEdgeHTM":  {filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"), filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe")},
		"ChromeHTML": {filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"), filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe")},
	}
	for key, paths := range candidates {
		if progID != key {
			continue
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p, true
			}
		}
	}
	return "", false
}
