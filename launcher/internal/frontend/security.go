package frontend

import (
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const runtimeEndpoint = "/wails/runtime"

func ResolveDevServer(raw string, production bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if production {
		return "", errors.New("production launcher must not use FRONTEND_DEVSERVER_URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("FRONTEND_DEVSERVER_URL must be a loopback HTTP root URL")
	}
	hostname := strings.TrimSpace(parsed.Hostname())
	ip := net.ParseIP(hostname)
	if !strings.EqualFold(hostname, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("FRONTEND_DEVSERVER_URL must use a loopback host")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("FRONTEND_DEVSERVER_URL must not include a path")
	}
	parsed.Path = "/"
	parsed.RawPath = ""
	return parsed.String(), nil
}

func SecurityMiddleware(devServerURL string) func(http.Handler) http.Handler {
	connectSources := "'self'"
	inlineScript := ""
	if parsed, err := url.Parse(devServerURL); err == nil && parsed.Host != "" {
		origin := parsed.Scheme + "://" + parsed.Host
		connectSources += " " + origin + " " + "ws://" + parsed.Host
		inlineScript = " 'unsafe-inline'"
	}
	contentSecurityPolicy := strings.Join([]string{
		"default-src 'none'",
		"base-uri 'none'",
		"child-src 'none'",
		"connect-src " + connectSources,
		"font-src 'self'",
		"form-action 'none'",
		"frame-ancestors 'none'",
		"frame-src 'none'",
		"img-src 'self' data:",
		"media-src 'none'",
		"object-src 'none'",
		"script-src 'self'" + inlineScript,
		"style-src 'self' 'unsafe-inline'",
		"worker-src 'none'",
	}, "; ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Security-Policy", contentSecurityPolicy)
			response.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			response.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			response.Header().Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), microphone=(), payment=(), usb=()")
			response.Header().Set("Referrer-Policy", "no-referrer")
			response.Header().Set("X-Content-Type-Options", "nosniff")
			if request.URL.Path == runtimeEndpoint && !trustedRuntimeRequest(request) {
				http.Error(response, "forbidden renderer request", http.StatusForbidden)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func trustedRuntimeRequest(request *http.Request) bool {
	if request.Method != http.MethodPost || strings.TrimSpace(request.Header.Get("X-Wails-Client-Id")) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false
	}
	if fetchSite := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site"))); fetchSite != "" && fetchSite != "same-origin" && fetchSite != "same-site" && fetchSite != "none" {
		return false
	}
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" || origin == "null" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "wails") {
		return false
	}
	return request.Host != "" && strings.EqualFold(parsed.Host, request.Host)
}
