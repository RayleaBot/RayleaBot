package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowOneBotIngressPrefersAuthorizationHeader(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/adapters/onebot11/webhook", nil)
	request.Header.Set("Authorization", "Bearer test-token")

	if !allowOneBotIngress(request, "test-token", false, true) {
		t.Fatal("expected Authorization bearer token to be accepted")
	}
}

func TestAllowOneBotIngressRejectsQueryTokenUnlessCompatibilityModeEnabled(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/adapters/onebot11/webhook?access_token=test-token", nil)

	if allowOneBotIngress(request, "test-token", false, true) {
		t.Fatal("expected query token to be rejected when compatibility mode is disabled")
	}
	if !allowOneBotIngress(request, "test-token", true, true) {
		t.Fatal("expected query token to be accepted when compatibility mode is enabled")
	}
}

func TestAllowOneBotIngressWithoutTokenRequiresLoopbackListener(t *testing.T) {
	for _, tt := range []struct {
		host    string
		allowed bool
	}{
		{"127.0.0.1", true},
		{"127.23.45.67", true},
		{"::1", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"::", false},
		{"192.168.1.2", false},
	} {
		t.Run(tt.host, func(t *testing.T) {
			handler := NewProtocolHandlers(nil, tt.host)
			request := httptest.NewRequest(http.MethodPost, "/api/adapters/onebot11/webhook", nil)
			if got := allowOneBotIngress(request, " ", false, handler.requireInboundToken); got != tt.allowed {
				t.Fatalf("allowOneBotIngress on %s = %v, want %v", tt.host, got, tt.allowed)
			}
		})
	}
}
