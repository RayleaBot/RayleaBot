package protocolapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowOneBotIngressPrefersAuthorizationHeader(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/protocols/onebot11/webhook", nil)
	request.Header.Set("Authorization", "Bearer test-token")

	if !allowOneBotIngress(request, "test-token", false) {
		t.Fatal("expected Authorization bearer token to be accepted")
	}
}

func TestAllowOneBotIngressRejectsQueryTokenUnlessCompatibilityModeEnabled(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/protocols/onebot11/webhook?access_token=test-token", nil)

	if allowOneBotIngress(request, "test-token", false) {
		t.Fatal("expected query token to be rejected when compatibility mode is disabled")
	}
	if !allowOneBotIngress(request, "test-token", true) {
		t.Fatal("expected query token to be accepted when compatibility mode is enabled")
	}
}
