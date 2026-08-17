package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestResolveServerEndpointNormalizesWildcardHost(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "user.yaml")
	writeTestFile(t, configPath, "server:\n  host: 0.0.0.0\n  port: \"9123\"\n")

	endpoint, warning := ResolveServerEndpoint(configPath)
	if endpoint.Host != "127.0.0.1" || endpoint.Port != 9123 || endpoint.BaseURL != "http://127.0.0.1:9123/" {
		t.Fatalf("ResolveServerEndpoint() = %#v", endpoint)
	}
	if warning != "" {
		t.Fatalf("ResolveServerEndpoint() warning = %q", warning)
	}
}

func TestResolveServerEndpointReportsInvalidConfigurationFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "user.yaml")
	writeTestFile(t, configPath, "server: [not-valid")

	endpoint, warning := ResolveServerEndpoint(configPath)
	if endpoint.Host != "127.0.0.1" || endpoint.Port != 8080 || warning == "" {
		t.Fatalf("ResolveServerEndpoint() = %#v, warning %q", endpoint, warning)
	}
}

func TestManagementClientUsesFormalLauncherControlHeader(t *testing.T) {
	token := "control-token"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/healthz":
			if request.Header.Get("X-Raylea-Launcher-Control") != "" {
				t.Error("healthz must not receive the launcher control token")
			}
			fmt.Fprint(writer, `{"status":"ok"}`)
		case "/readyz":
			writer.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(writer, `{"status":"setup_required"}`)
		case "/api/launcher/status":
			if request.Header.Get("X-Raylea-Launcher-Control") != token {
				t.Errorf("status control header = %q", request.Header.Get("X-Raylea-Launcher-Control"))
			}
			fmt.Fprint(writer, `{"status":"ready"}`)
		case "/api/launcher/shutdown":
			if request.Method != http.MethodPost || request.Header.Get("X-Raylea-Launcher-Control") != token {
				t.Errorf("shutdown request = %s token=%q", request.Method, request.Header.Get("X-Raylea-Launcher-Control"))
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	endpoint := ServerEndpoint{BaseURL: server.URL + "/"}
	client := NewManagementClient(func() string { return token })
	if !client.IsHealthy(context.Background(), endpoint) {
		t.Fatal("IsHealthy() = false")
	}
	readiness, err := client.GetReadiness(context.Background(), endpoint)
	if err != nil || objectStatus(readiness) != "setup_required" {
		t.Fatalf("GetReadiness() = %#v, %v", readiness, err)
	}
	status, err := client.GetLauncherStatus(context.Background(), endpoint)
	if err != nil || objectStatus(status) != "ready" {
		t.Fatalf("GetLauncherStatus() = %#v, %v", status, err)
	}
	if err := client.Shutdown(context.Background(), endpoint); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestManagementClientReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"error":{"code":"launcher.control_denied","message":"denied"}}`)
	}))
	defer server.Close()

	_, err := NewManagementClient(func() string { return "bad" }).GetLauncherStatus(context.Background(), ServerEndpoint{BaseURL: server.URL + "/"})
	if err == nil || err.Error() != "launcher.control_denied: denied" {
		t.Fatalf("GetLauncherStatus() error = %v", err)
	}
}
