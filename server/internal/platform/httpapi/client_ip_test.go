package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestRequestRemoteIPUsesDirectPeer(t *testing.T) {
	request := httptest.NewRequest("GET", "http://192.168.1.50:8080/", nil)
	request.RemoteAddr = "192.168.1.20:32000"
	request.Header.Set("Forwarded", "for=203.0.113.1")
	request.Header.Set("X-Forwarded-For", "203.0.113.2")
	request.Header.Set("X-Real-IP", "203.0.113.3")
	if got := RequestRemoteIP(request); got != "192.168.1.20" {
		t.Fatalf("client IP=%q", got)
	}
}
