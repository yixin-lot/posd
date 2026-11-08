//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ensureFirewallRule 为当前 posd.exe 添加 Windows 防火墙入站放行规则。
// 普通用户下会弹一次 UAC，用户点“是”即可；已存在则直接返回成功。
func ensureFirewallRule(exe string) (bool, string) {
	if firewallRuleExists() {
		return true, "防火墙规则已存在"
	}
	ps := fmt.Sprintf(`$arg = @('advfirewall','firewall','add','rule','name=Posd','dir=in','action=allow','program="%s"','enable=yes','profile=private,domain'); $p = Start-Process -FilePath netsh -ArgumentList $arg -Verb RunAs -PassThru -Wait; exit $p.ExitCode`, exe)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return false, "添加失败（可能取消了管理员确认）：" + msg
	}
	time.Sleep(500 * time.Millisecond)
	if firewallRuleExists() {
		return true, "已放行 posd"
	}
	return false, "规则未生效，请手动放行"
}

func firewallRuleExists() bool {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=Posd")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Posd")
}

var _ = os.Getpid
