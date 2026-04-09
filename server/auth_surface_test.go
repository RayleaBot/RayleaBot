package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthShellDoesNotAddPublicRoutes(t *testing.T) {
	t.Parallel()

	application := newTestApp(t)
	if application.AuthManager() == nil {
		t.Fatalf("expected auth manager to be initialized")
	}

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodPost, path: "/api/setup/admin", want: http.StatusBadRequest},
		{method: http.MethodGet, path: "/api/setup/status", want: http.StatusOK},
		{method: http.MethodPost, path: "/api/session/login", want: http.StatusBadRequest},
		{method: http.MethodPost, path: "/api/session/launcher-token", want: http.StatusForbidden},
		{method: http.MethodPost, path: "/api/session/launcher-admission", want: http.StatusForbidden},
		{method: http.MethodDelete, path: "/api/session", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/config", want: http.StatusUnauthorized},
		{method: http.MethodPut, path: "/api/config", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/system/status", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/system/shutdown", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/logs", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/logs/log_test_0001", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/tasks", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/install", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/raylea.help/enable", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/raylea.help/disable", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/events", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/tasks", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/logs", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/plugins/raylea.help/console", want: http.StatusUnauthorized},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		request.RemoteAddr = "127.0.0.1:0"
		recorder := httptest.NewRecorder()
		application.Handler().ServeHTTP(recorder, request)

		if recorder.Code != tc.want {
			t.Fatalf("unexpected status for %s %s: got %d want %d", tc.method, tc.path, recorder.Code, tc.want)
		}
	}
}
