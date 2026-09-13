package httpapi

import (
	"net"
	"net/http"
	"strings"
)

func remoteAddressIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	host := strings.TrimSpace(request.RemoteAddr)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(host, "[]")))
	if ip != nil {
		return ip.String()
	}
	return strings.TrimSpace(strings.Trim(host, "[]"))
}
