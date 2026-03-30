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
	if hasForwardingHeaders(r) {
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

func hasForwardingHeaders(r *http.Request) bool {
	for _, header := range []string{
		"Forwarded",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Port",
		"X-Forwarded-Proto",
		"X-Real-IP",
	} {
		if strings.TrimSpace(r.Header.Get(header)) != "" {
			return true
		}
	}

	return false
}
