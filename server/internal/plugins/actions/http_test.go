package actions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestExecuteHTTPSendsExplicitRequestAndReturnsText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request"); got != "fixture" {
			t.Fatalf("unexpected explicit header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	result, err := executeHTTPRequest(context.Background(), "plugin.http", plugins.Action{
		HTTPMethod:  "GET",
		HTTPURL:     server.URL + "/v1/data",
		HTTPHeaders: map[string]string{"X-Request": "fixture"},
	}, config.Config{
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    5,
			MaxRetries:        0,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
	}, stubHTTPActionPermissions{
		permissions: map[string]bool{"http.request": true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequest failed: %v", err)
	}
	if result["status_code"] != http.StatusOK || result["body_text"] != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteHTTPAllowsConfiguredPrivateHost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	result, err := executeHTTPRequest(context.Background(), "plugin.http", plugins.Action{
		HTTPMethod: "GET",
		HTTPURL:    server.URL + "/cover.jpg",
	}, config.Config{
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    5,
			MaxRetries:        0,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
	}, stubHTTPActionPermissions{
		permissions: map[string]bool{"http.request": true},
	})
	if err != nil {
		t.Fatalf("executeHTTPRequest failed: %v", err)
	}
	if result["status_code"] != http.StatusOK || result["body_text"] != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteHTTPRejectsPrivateHostWithoutServerAllowlist(t *testing.T) {
	t.Parallel()

	_, err := executeHTTPRequest(context.Background(), "plugin.http", plugins.Action{
		HTTPMethod: "GET",
		HTTPURL:    "https://127.0.0.1/v1/data",
	}, config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 5, MaxRetries: 0}}, stubHTTPActionPermissions{
		permissions: map[string]bool{"http.request": true},
	})

	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

func TestExecuteHTTPMapsOversizedResponseToStableError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("response is too large"))
	}))
	defer server.Close()

	_, err := executeHTTPRequest(context.Background(), "plugin.http", plugins.Action{
		HTTPMethod: "GET",
		HTTPURL:    server.URL,
	}, config.Config{HTTP: config.HTTPConfig{
		TimeoutSeconds:       5,
		MaxResponseBodyBytes: 4,
		AllowPrivateHosts:    []string{"127.0.0.1"},
	}}, stubHTTPActionPermissions{
		permissions: map[string]bool{"http.request": true},
	})

	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.upstream_response_too_large" {
		t.Fatalf("unexpected oversized response error: %#v", err)
	}
}

type stubHTTPActionPermissions struct {
	permissions map[string]bool
}

func (s stubHTTPActionPermissions) PermissionDeclared(_ context.Context, _ string, permission string) bool {
	return s.permissions[permission]
}

func (s stubHTTPActionPermissions) PermissionPlatforms(context.Context, string, string) []string {
	return nil
}

func (s stubHTTPActionPermissions) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}
