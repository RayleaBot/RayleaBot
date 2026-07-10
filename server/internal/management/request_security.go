package management

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func validSetupRequest(r *http.Request, cfg AuthConfig) bool {
	if r == nil || !validRequestHost(r, cfg.AllowedHosts) || !validRequestOrigin(r, cfg.AllowedOrigins, true) {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "", "same-origin", "none":
	default:
		return false
	}

	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Mode"))) {
	case "", "cors", "same-origin":
		return true
	default:
		return false
	}
}

func validRequestHost(r *http.Request, allowed []string) bool {
	if r == nil {
		return false
	}
	host, ok := normalizeAuthority(r.Host)
	if !ok {
		return false
	}
	if len(allowed) == 0 {
		return host != ""
	}
	for _, candidate := range allowed {
		if normalized, valid := normalizeAuthority(candidate); valid {
			if normalized == host {
				return true
			}
		}
	}
	return false
}

func validRequestOrigin(r *http.Request, allowed []string, required bool) bool {
	if r == nil {
		return false
	}
	rawOrigin := strings.TrimSpace(r.Header.Get("Origin"))
	if rawOrigin == "" {
		return !required
	}
	origin, ok := normalizeOrigin(rawOrigin)
	if !ok {
		return false
	}
	if len(allowed) == 0 {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		requestOrigin, valid := normalizeOrigin(scheme + "://" + r.Host)
		return valid && origin == requestOrigin
	}
	for _, candidate := range allowed {
		if normalized, valid := normalizeOrigin(candidate); valid && normalized == origin {
			return true
		}
	}
	return false
}

func normalizedOriginAuthority(rawOrigin string) (string, bool) {
	origin, ok := normalizeOrigin(rawOrigin)
	if !ok {
		return "", false
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", false
	}
	return normalizeAuthority(parsed.Host)
}

func normalizeOrigin(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Host == "" {
		return "", false
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	authority, ok := normalizeAuthority(parsed.Host)
	if !ok {
		return "", false
	}
	return strings.ToLower(parsed.Scheme) + "://" + authority, true
}

func normalizeAuthority(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "/\\?#@") {
		return "", false
	}

	host := raw
	port := ""
	if parsedHost, parsedPort, err := net.SplitHostPort(raw); err == nil {
		host = parsedHost
		port = parsedPort
		parsedPortNumber, portErr := strconv.Atoi(port)
		if portErr != nil || parsedPortNumber < 1 || parsedPortNumber > 65535 {
			return "", false
		}
	} else if strings.Count(raw, ":") > 1 {
		if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
			return "", false
		}
		host = strings.Trim(raw, "[]")
	}
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		for _, label := range strings.Split(host, ".") {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return "", false
			}
			for _, character := range label {
				if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
					return "", false
				}
			}
		}
	}
	if port != "" {
		return net.JoinHostPort(host, port), true
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]", true
	}
	return host, true
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}
