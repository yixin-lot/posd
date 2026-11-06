package print

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// TCPPrinter 是走 ESC/POS raw 的网口打印机（默认 9100 端口）。
//
// 为什么需要它：厨房打印机常常不在收银机旁边，而 Windows 队列
// 要么要求装驱动、要么要求装在本机。网口打印机本身就实现了
// ESC/POS raw 协议，直接写 socket 即可，跨平台、免驱动。
type TCPPrinter struct {
	Addr    string // host:port
	Timeout time.Duration
}

// NewTCPPrinter 从 `tcp://host:port` 或 `host:port` 构造。
func NewTCPPrinter(raw string) (*TCPPrinter, error) {
	addr, err := ParseTCPAddr(raw)
	if err != nil {
		return nil, err
	}
	return &TCPPrinter{Addr: addr, Timeout: 3 * time.Second}, nil
}

// Name 实现 Printer。
func (p *TCPPrinter) Name() string { return "tcp://" + p.Addr }

// Write 实现 Printer：一次连接写完整包，失败重试一次（网络抖动常见）。
func (p *TCPPrinter) Write(payload []byte) error {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		conn, err := net.DialTimeout("tcp", p.Addr, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
		_, err = conn.Write(payload)
		closeErr := conn.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("连接打印机 %s 失败: %w", p.Addr, lastErr)
}

// ParseTCPAddr 校验并规范化地址（接受带或不带 tcp:// 前缀）。
func ParseTCPAddr(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "tcp://")
	if s == "" {
		return "", fmt.Errorf("打印机地址为空")
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return "", fmt.Errorf("打印机地址应形如 tcp://192.168.1.50:9100（%s）", raw)
	}
	if host == "" {
		return "", fmt.Errorf("打印机地址缺少主机")
	}
	if port == "" {
		return "", fmt.Errorf("打印机地址缺少端口（通常 9100）")
	}
	return net.JoinHostPort(host, port), nil
}

// IsTCP 判断一个打印机名是否是网口地址。
func IsTCP(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "tcp://")
}

// Resolve 按名字返回打印机：tcp:// 走 socket，其余当 Windows 队列名。
func Resolve(name string) (Printer, error) {
	if IsTCP(name) {
		return NewTCPPrinter(name)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("打印机名为空")
	}
	return &Spooler{Printer: name}, nil
}
