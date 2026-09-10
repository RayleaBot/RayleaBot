package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyResolverUsesFirstUntrustedAddressFromRight(t *testing.T) {
	t.Parallel()

	resolver := NewTrustedProxyResolver("public_via_reverse_proxy", []string{"127.0.0.0/8", "10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodPost, "/api/session/login", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("X-Forwarded-For", "198.51.100.25, 10.2.3.4")

	var clientIP, peerIP string
	var trusted bool
	resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		clientIP = RequestRemoteIP(request)
		peerIP = RequestPeerIP(request)
		trusted = RequestUsesTrustedProxy(request)
	})).ServeHTTP(httptest.NewRecorder(), request)

	if clientIP != "198.51.100.25" || peerIP != "127.0.0.1" || !trusted {
		t.Fatalf("unexpected proxy resolution: client=%q peer=%q trusted=%v", clientIP, peerIP, trusted)
	}
}

func TestTrustedProxyResolverParsesForwardedIPv6(t *testing.T) {
	t.Parallel()

	resolver := NewTrustedProxyResolver("public_via_reverse_proxy", []string{"127.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Forwarded", `for="[2001:db8::7]:4711";proto=https`)

	var clientIP string
	resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		clientIP = RequestRemoteIP(request)
	})).ServeHTTP(httptest.NewRecorder(), request)
	if clientIP != "2001:db8::7" {
		t.Fatalf("Forwarded client IP = %q", clientIP)
	}
}

func TestTrustedProxyResolverIgnoresSpoofedHeadersFromUntrustedPeer(t *testing.T) {
	t.Parallel()

	resolver := NewTrustedProxyResolver("public_via_reverse_proxy", []string{"127.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.44:54321"
	request.Header.Set("X-Forwarded-For", "198.51.100.25")

	var clientIP string
	var trusted bool
	resolver.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		clientIP = RequestRemoteIP(request)
		trusted = RequestUsesTrustedProxy(request)
	})).ServeHTTP(httptest.NewRecorder(), request)
	if clientIP != "192.0.2.44" || trusted {
		t.Fatalf("spoofed header was trusted: client=%q trusted=%v", clientIP, trusted)
	}
}
