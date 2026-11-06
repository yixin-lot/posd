//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 托盘图标：原生 Win32 Shell_NotifyIcon，不引 CGO / 第三方库。
// 菜单：打开收银台 / 退出；左键双击也能打开。

var (
	trayUser32   = windows.NewLazySystemDLL("user32.dll")
	trayShell32  = windows.NewLazySystemDLL("shell32.dll")
	trayKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW  = trayUser32.NewProc("RegisterClassExW")
	pCreateWindowExW   = trayUser32.NewProc("CreateWindowExW")
	pDefWindowProcW    = trayUser32.NewProc("DefWindowProcW")
	pDestroyWindow     = trayUser32.NewProc("DestroyWindow")
	pGetMessageW       = trayUser32.NewProc("GetMessageW")
	pTranslateMessage  = trayUser32.NewProc("TranslateMessage")
	pDispatchMessageW  = trayUser32.NewProc("DispatchMessageW")
	pPostQuitMessage   = trayUser32.NewProc("PostQuitMessage")
	pPostMessageW      = trayUser32.NewProc("PostMessageW")
	pDestroyMenu       = trayUser32.NewProc("DestroyMenu")
	pCreatePopupMenu   = trayUser32.NewProc("CreatePopupMenu")
	pAppendMenuW       = trayUser32.NewProc("AppendMenuW")
	pTrackPopupMenu    = trayUser32.NewProc("TrackPopupMenu")
	pSetForegroundWin  = trayUser32.NewProc("SetForegroundWindow")
	pGetCursorPos      = trayUser32.NewProc("GetCursorPos")
	pLoadIconW         = trayUser32.NewProc("LoadIconW")
	pShellNotifyIconW  = trayShell32.NewProc("Shell_NotifyIconW")
	pGetModuleHandleW  = trayKernel32.NewProc("GetModuleHandleW")
	pRegisterWindowMsg = trayUser32.NewProc("RegisterWindowMessageW")
)

const (
	wmApp        = 0x8000
	wmTray       = wmApp + 1
	wmClose      = 0x0010
	wmLButtonUp  = 0x0202
	wmLButtonDbl = 0x0203
	wmRButtonUp  = 0x0205

	nimAdd    = 0
	nimDelete = 2
	nifMsg    = 1
	nifIcon   = 2
	nifTip    = 4
	idiApp    = 32512

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	mfString       = 0x0000
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

type msgT struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type pointT struct{ X, Y int32 }

const (
	menuOpen = 1
	menuQuit = 2
)

// startTray 创建托盘图标并进入消息循环（内部 goroutine，调用即返回）。
func startTray(tooltip string, onOpen, onQuit func()) error {
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := runTray(tooltip, onOpen, onQuit, ready); err != nil {
			ready <- err
		}
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(3 * time.Second):
		return fmt.Errorf("托盘初始化超时")
	}
}

func runTray(tooltip string, onOpen, onQuit func(), ready chan<- error) error {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("PosdTrayWnd")

	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   windows.NewCallback(func(hwnd, msg, wp, lp uintptr) uintptr { return trayProc(hwnd, msg, wp, lp, onOpen, onQuit) }),
		Instance:  hInst,
		ClassName: className,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %v", err)
	}

	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %v", err)
	}

	// 优先用 exe 内嵌图标（资源 ID 1），取不到再退回系统默认
	icon, _, _ := pLoadIconW.Call(hInst, 1)
	if icon == 0 {
		icon, _, _ = pLoadIconW.Call(0, idiApp)
	}
	nid := notifyIconData{Wnd: hwnd, ID: 1, Flags: nifMsg | nifIcon | nifTip, CallbackMessage: wmTray, Icon: icon}
	nid.Size = uint32(unsafe.Sizeof(nid))
	copy(nid.Tip[:], windows.StringToUTF16(tooltip))

	if r, _, err := pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return fmt.Errorf("Shell_NotifyIcon add: %v", err)
	}
	defer pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))

	// explorer.exe 重启后托盘会被清空，系统会广播 TaskbarCreated，收到后重新挂图标
	taskbarMsg, _, _ := pRegisterWindowMsg.Call(uintptr(unsafe.Pointer(mustUTF16("TaskbarCreated"))))

	ready <- nil

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return nil
		}
		if taskbarMsg != 0 && m.Message == uint32(taskbarMsg) {
			pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func mustUTF16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func trayProc(hwnd, msg, wp, lp uintptr, onOpen, onQuit func()) uintptr {
	switch msg {
	case wmTray:
		switch lp {
		case wmLButtonDbl:
			if onOpen != nil {
				onOpen()
			}
		case wmRButtonUp:
			pSetForegroundWin.Call(hwnd)
			menu, _, _ := pCreatePopupMenu.Call()
			openTxt, _ := windows.UTF16PtrFromString("打开收银台")
			quitTxt, _ := windows.UTF16PtrFromString("退出")
			pAppendMenuW.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(openTxt)))
			pAppendMenuW.Call(menu, mfString, menuQuit, uintptr(unsafe.Pointer(quitTxt)))
			var pt pointT
			pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
			cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
			pDestroyMenu.Call(menu)
			switch cmd {
			case menuOpen:
				if onOpen != nil {
					onOpen()
				}
			case menuQuit:
				pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Wnd: hwnd, ID: 1})))
				if onQuit != nil {
					onQuit()
				}
				pPostQuitMessage.Call(0)
				pDestroyWindow.Call(hwnd)
			}
		}
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

var _ = syscall.Getpid
