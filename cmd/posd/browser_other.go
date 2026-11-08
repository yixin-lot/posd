//go:build !windows

package main

import "os/exec"

var appBrowser *exec.Cmd

func openAppWindow(url string) (*exec.Cmd, error) { return nil, exec.ErrNotFound }
func closeAppWindow()                              {}
