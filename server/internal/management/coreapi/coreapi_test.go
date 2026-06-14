package coreapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLoopbackRequestRejectsForwardedHeaders(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/launcher/status", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "127.0.0.1")

	if IsLoopbackRequest(request) {
		t.Fatalf("expected forwarded loopback request to be rejected")
	}
}

func TestIsLoopbackRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		want       bool
	}{
		{name: "ipv4 loopback", remoteAddr: "127.0.0.1:12345", want: true},
		{name: "ipv6 loopback", remoteAddr: "[::1]:12345", want: true},
		{name: "localhost", remoteAddr: "localhost:12345", want: true},
		{name: "public host", remoteAddr: "203.0.113.9:12345", want: false},
		{name: "empty", remoteAddr: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "/api/launcher/status", nil)
			request.RemoteAddr = tc.remoteAddr

			if got := IsLoopbackRequest(request); got != tc.want {
				t.Fatalf("IsLoopbackRequest(%q) = %v, want %v", tc.remoteAddr, got, tc.want)
			}
		})
	}
}
