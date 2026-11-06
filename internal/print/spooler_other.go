//go:build !windows

package print

import "errors"

// Spooler 在非 Windows 平台不可用。
type Spooler struct{ Printer string }

// Name 返回通道名。
func (s *Spooler) Name() string { return "spooler:" + s.Printer }

// Write 始终返回不支持。
func (s *Spooler) Write([]byte) error { return errors.New("当前平台不支持 spooler 通道") }
