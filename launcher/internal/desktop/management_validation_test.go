package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagementRejectsInvalidContractAtHTTPBoundary(t *testing.T) {
	cases := []struct{ name, path, payload string }{
		{"missing status", "/readyz", `{}`},
		{"empty status", "/readyz", `{"status":""}`},
		{"invalid readiness enum", "/readyz", `{"status":"future"}`},
		{"null response", "/readyz", `null`},
		{"unknown property", "/readyz", `{"status":"ready","legacy":true}`},
		{"invalid nested severity", "/readyz", `{"status":"degraded","issues":[{"code":"x","severity":"fatal","summary":"x"}]}`},
		{"empty runtime resources", "/readyz", `{"status":"degraded","issues":[{"code":"x","severity":"error","summary":"x","runtime_resources":[]}]}`},
		{"multiple JSON values", "/readyz", `{"status":"ready"}{}`},
		{"missing adapters", "/api/launcher/status", `{"status":"running"}`},
		{"negative count", "/api/launcher/status", `{"status":"running","adapters":[],"active_plugins":-1}`},
		{"fractional count", "/api/launcher/status", `{"status":"running","adapters":[],"active_plugins":0.5}`},
		{"unknown adapter state", "/api/launcher/status", `{"status":"running","adapters":[{"id":"a","protocol":"onebot11","enabled":true,"state":"future"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, c.payload) }))
			defer server.Close()
			client := NewManagementClient(func() string { return "fixture" })
			endpoint := ServerEndpoint{BaseURL: server.URL + "/"}
			var err error
			if c.path == "/readyz" {
				_, err = client.GetReadiness(context.Background(), endpoint)
			} else {
				_, err = client.GetLauncherStatus(context.Background(), endpoint)
			}
			var boundary *BoundaryError
			if !errors.As(err, &boundary) || boundary.Code != "launcher.invalid_server_response" {
				t.Fatalf("invalid response escaped: %v", err)
			}
		})
	}
}

func TestManagementRetainsOpenDisplayCodeAndBoundsResponse(t *testing.T) {
	payload := `{"status":"degraded","issues":[{"code":"future.display_diagnostic","severity":"warning","summary":"future diagnostic"}]}`
	value, err := decodeServerResponse[ServerReadinessStatusResponse](strings.NewReader(payload), "ReadinessStatusResponse")
	if err != nil || value.Issues[0].Code != "future.display_diagnostic" {
		t.Fatalf("open display code rejected: %#v %v", value, err)
	}
	_, err = decodeServerResponse[ServerReadinessStatusResponse](strings.NewReader(strings.Repeat(" ", maxManagementResponseBytes+1)), "ReadinessStatusResponse")
	var boundary *BoundaryError
	if !errors.As(err, &boundary) || boundary.Code != "launcher.response_too_large" {
		t.Fatalf("oversize response accepted: %v", err)
	}
}

func TestLauncherClientAddressSharedVectors(t *testing.T) {
	payload, err := os.ReadFile("../../../server/testdata/client-addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Listen, URL string }
	if err := json.Unmarshal(payload, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Listen, func(t *testing.T) {
			host, port, err := net.SplitHostPort(c.Listen)
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(t.TempDir(), "user.yaml")
			writeTestFile(t, config, fmt.Sprintf("server:\n  host: %q\n  port: %s\n", host, port))
			endpoint, warning := ResolveServerEndpoint(config)
			if warning != "" || strings.TrimSuffix(endpoint.BaseURL, "/") != c.URL {
				t.Fatalf("endpoint=%#v warning=%q want=%s", endpoint, warning, c.URL)
			}
		})
	}
}
