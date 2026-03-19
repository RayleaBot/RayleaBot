package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthShellDoesNotAddPublicRoutes(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	if application.Auth == nil {
		t.Fatalf("expected auth manager to be initialized")
	}

	cases := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/session/login"},
		{method: http.MethodDelete, path: "/api/session"},
		{method: http.MethodPost, path: "/api/session/launcher-token"},
		{method: http.MethodGet, path: "/ws/events"},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		recorder := httptest.NewRecorder()
		application.Handler().ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s %s to remain unimplemented, got status %d", tc.method, tc.path, recorder.Code)
		}
	}
}
