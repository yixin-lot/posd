//go:build !windows

package main

// 非 Windows 平台没有系统托盘：保持空实现，桌面版逻辑照常。
func startTray(tooltip string, onOpen, onQuit func()) error { return nil }
