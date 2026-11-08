//go:build !windows

package main

func ensureFirewallRule(exe string) (bool, string) { return false, "当前系统不支持自动放行" }
