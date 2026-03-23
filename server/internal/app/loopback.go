package app

import (
	"net"
	"net/http"
	"strings"
)

func isLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return false
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
