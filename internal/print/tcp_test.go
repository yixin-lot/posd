package print

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestTCPPrinterWritesBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer ln.Close()

	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		n, _ := io.ReadAtLeast(conn, buf, 1)
		got <- buf[:n]
	}()

	p, err := NewTCPPrinter("tcp://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if err := p.Write([]byte("HELLO-ESC-POS")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	select {
	case b := <-got:
		if string(b) != "HELLO-ESC-POS" {
			t.Fatalf("收到 %q", string(b))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时未收到数据")
	}
}

func TestParseTCPAddr(t *testing.T) {
	cases := []struct {
		in   string
		want string
		bad  bool
	}{
		{"tcp://192.168.1.50:9100", "192.168.1.50:9100", false},
		{"192.168.1.50:9100", "192.168.1.50:9100", false},
		{"tcp://printer.local:9100", "printer.local:9100", false},
		{"tcp://192.168.1.50", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := ParseTCPAddr(c.in)
		if c.bad {
			if err == nil {
				t.Fatalf("%q 应报错", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("%q -> %q, %v", c.in, got, err)
		}
	}
}

func TestResolvePicksDriver(t *testing.T) {
	p, err := Resolve("tcp://192.168.1.50:9100")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.HasPrefix(p.Name(), "tcp://") {
		t.Fatalf("应返回 TCP 驱动，实际 %s", p.Name())
	}

	sp, err := Resolve("POS-58")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.HasSuffix(sp.Name(), "POS-58") {
		t.Fatalf("应返回队列驱动，实际 %s", sp.Name())
	}
}
