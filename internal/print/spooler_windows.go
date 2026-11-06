//go:build windows

package print

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Spooler 通过 Windows 打印后台（winspool.drv）以 RAW 方式直发字节。
// 用 stdlib 的 syscall 调用，不引入 cgo，保持单文件交付。
type Spooler struct {
	Printer string
}

type docInfo1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

var (
	winspool          = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinter   = winspool.NewProc("OpenPrinterW")
	procClosePrinter  = winspool.NewProc("ClosePrinter")
	procStartDoc      = winspool.NewProc("StartDocPrinterW")
	procStartPage     = winspool.NewProc("StartPagePrinter")
	procWritePrinter  = winspool.NewProc("WritePrinter")
	procEndPage       = winspool.NewProc("EndPagePrinter")
	procEndDoc        = winspool.NewProc("EndDocPrinter")
)

// Name 返回通道名。
func (s *Spooler) Name() string { return "spooler:" + s.Printer }

// Write 把字节原样发给打印机（不做任何编码或排版处理）。
func (s *Spooler) Write(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	name, err := syscall.UTF16PtrFromString(s.Printer)
	if err != nil {
		return fmt.Errorf("打印机名不合法: %w", err)
	}
	var handle uintptr
	if ret, _, callErr := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0); ret == 0 {
		return fmt.Errorf("打开打印机失败: %v", callErr)
	}
	defer func() { _, _, _ = procClosePrinter.Call(handle) }()

	docName, _ := syscall.UTF16PtrFromString("POS Receipt")
	dataType, _ := syscall.UTF16PtrFromString("RAW")
	info := docInfo1{pDocName: docName, pDatatype: dataType}
	if ret, _, callErr := procStartDoc.Call(handle, 1, uintptr(unsafe.Pointer(&info))); ret == 0 {
		return fmt.Errorf("创建打印作业失败: %v", callErr)
	}
	if ret, _, callErr := procStartPage.Call(handle); ret == 0 {
		_, _, _ = procEndDoc.Call(handle)
		return fmt.Errorf("开始打印页失败: %v", callErr)
	}

	var written uint32
	ret, _, callErr := procWritePrinter.Call(handle, uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)),
		uintptr(unsafe.Pointer(&written)))
	if ret == 0 {
		_, _, _ = procEndPage.Call(handle)
		_, _, _ = procEndDoc.Call(handle)
		return fmt.Errorf("写入打印数据失败: %v", callErr)
	}
	_, _, _ = procEndPage.Call(handle)
	if ret, _, callErr := procEndDoc.Call(handle); ret == 0 {
		return fmt.Errorf("结束打印作业失败: %v", callErr)
	}
	if int(written) != len(payload) {
		return fmt.Errorf("只写入 %d/%d 字节", written, len(payload))
	}
	return nil
}
