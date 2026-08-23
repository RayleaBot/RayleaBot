package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type clientIPContextKey struct{}

type clientIPContext struct {
	peerIP       string
	clientIP     string
	trustedProxy bool
}

type TrustedProxyResolver struct {
	enabled  bool
	networks []*net.IPNet
}

func NewTrustedProxyResolver(exposureMode string, cidrs []string) *TrustedProxyResolver {
	resolver := &TrustedProxyResolver{enabled: strings.TrimSpace(exposureMode) == "public_via_reverse_proxy"}
	if !resolver.enabled {
		return resolver
	}
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil {
			resolver.networks = append(resolver.networks, network)
		}
	}
	return resolver
}

func (resolver *TrustedProxyResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		peerIP := remoteAddressIP(request)
		info := clientIPContext{peerIP: peerIP, clientIP: peerIP}
		if resolver != nil && resolver.enabled && resolver.contains(peerIP) {
			info.trustedProxy = true
			if clientIP := resolver.forwardedClientIP(request); clientIP != "" {
				info.clientIP = clientIP
			}
		}
		ctx := context.WithValue(request.Context(), clientIPContextKey{}, info)
		next.ServeHTTP(w, request.WithContext(ctx))
	})
}

func (resolver *TrustedProxyResolver) forwardedClientIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	candidates := parseForwardedFor(request.Header.Values("Forwarded"))
	if len(candidates) == 0 {
		candidates = parseXForwardedFor(request.Header.Values("X-Forwarded-For"))
	}
	if len(candidates) == 0 {
		if value := parseIPToken(request.Header.Get("X-Real-IP")); value != "" {
			candidates = []string{value}
		}
	}
	for index := len(candidates) - 1; index >= 0; index-- {
		if !resolver.contains(candidates[index]) {
			return candidates[index]
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func (resolver *TrustedProxyResolver) contains(rawIP string) bool {
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(rawIP, "[]")))
	if ip == nil {
		return false
	}
	for _, network := range resolver.networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseForwardedFor(values []string) []string {
	items := make([]string, 0)
	for _, value := range values {
		for _, element := range strings.Split(value, ",") {
			for _, parameter := range strings.Split(element, ";") {
				name, raw, ok := strings.Cut(parameter, "=")
				if !ok || !strings.EqualFold(strings.TrimSpace(name), "for") {
					continue
				}
				if parsed := parseIPToken(raw); parsed != "" {
					items = append(items, parsed)
				}
				break
			}
		}
	}
	return items
}

func parseXForwardedFor(values []string) []string {
	items := make([]string, 0)
	for _, value := range values {
		for _, raw := range strings.Split(value, ",") {
			if parsed := parseIPToken(raw); parsed != "" {
				items = append(items, parsed)
			}
		}
	}
	return items
}

func parseIPToken(raw string) string {
	value := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), `"`))
	if value == "" || strings.EqualFold(value, "unknown") || strings.HasPrefix(value, "_") {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if end := strings.Index(value, "]"); end > 0 {
			value = value[1:end]
		}
	} else if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	ip := net.ParseIP(strings.TrimSpace(strings.Trim(value, "[]")))
	if ip == nil {
		return ""
	}
	return ip.String()
}

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

func RequestPeerIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	if info, ok := request.Context().Value(clientIPContextKey{}).(clientIPContext); ok && info.peerIP != "" {
		return info.peerIP
	}
	return remoteAddressIP(request)
}

func RequestUsesTrustedProxy(request *http.Request) bool {
	if request == nil {
		return false
	}
	info, ok := request.Context().Value(clientIPContextKey{}).(clientIPContext)
	return ok && info.trustedProxy
}
