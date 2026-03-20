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
		want   int
	}{
		{method: http.MethodPost, path: "/api/setup/admin", want: http.StatusBadRequest},
		{method: http.MethodGet, path: "/api/setup/status", want: http.StatusOK},
		{method: http.MethodPost, path: "/api/session/login", want: http.StatusBadRequest},
		{method: http.MethodDelete, path: "/api/session", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/session/launcher-token", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/system/status", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/system/shutdown", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/api/tasks", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/install", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/hello-node/enable", want: http.StatusUnauthorized},
		{method: http.MethodPost, path: "/api/plugins/hello-node/disable", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/events", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/tasks", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/logs", want: http.StatusUnauthorized},
		{method: http.MethodGet, path: "/ws/plugins/hello-node/console", want: http.StatusUnauthorized},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		recorder := httptest.NewRecorder()
		application.Handler().ServeHTTP(recorder, request)

		if recorder.Code != tc.want {
			t.Fatalf("unexpected status for %s %s: got %d want %d", tc.method, tc.path, recorder.Code, tc.want)
		}
	}
}
