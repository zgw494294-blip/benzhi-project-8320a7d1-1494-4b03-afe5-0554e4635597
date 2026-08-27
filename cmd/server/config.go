package main

import (
	"net"
	"strconv"
	"strings"
)

func safeAddress(addr string) bool {
	if strings.TrimSpace(addr) == "" {
		return false
	}
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil || (host != "127.0.0.1" && host != "localhost") {
		return false
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 || port == 80 || port == 8080 || port == 3000 {
		return false
	}
	return true
}
