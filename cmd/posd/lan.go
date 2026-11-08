package main

import (
	"net"
	"strings"
)

// lanURLs 把监听地址换成本机各网卡的局域网访问地址（手机用）。
func lanURLs(addr string) []string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || strings.HasPrefix(ip4.String(), "169.254.") {
				continue
			}
			out = append(out, "http://"+ip4.String()+":"+port+"/")
		}
	}
	return out
}
