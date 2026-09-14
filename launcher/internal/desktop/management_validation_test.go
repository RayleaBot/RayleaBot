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

func TestManagementDecodesServerResponsesLeniently(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			fmt.Fprint(w, `{"status":"future","legacy":true,"issues":[{"code":"x","severity":"fatal","summary":"x"}]}`)
			return
		}
		fmt.Fprint(w, `{"status":"running","adapters":[{"id":"a","protocol":"future","enabled":true,"state":"future"}],"new_field":1}`)
	}))
	defer server.Close()
	client := NewManagementClient(func() string { return "fixture" })
	endpoint := ServerEndpoint{BaseURL: server.URL + "/"}
	readiness, err := client.GetReadiness(context.Background(), endpoint)
	if err != nil || readiness.Status != "future" || readiness.Issues[0].Severity != "fatal" {
		t.Fatalf("readiness = %#v, %v", readiness, err)
	}
	status, err := client.GetLauncherStatus(context.Background(), endpoint)
	if err != nil || status.Adapters[0].State != "future" {
		t.Fatalf("status = %#v, %v", status, err)
	}
}

func TestManagementRetainsOpenDisplayCodeAndBoundsResponse(t *testing.T) {
	payload := `{"status":"degraded","issues":[{"code":"future.display_diagnostic","severity":"warning","summary":"future diagnostic"}]}`
	value, err := decodeServerResponse[ServerReadinessStatusResponse](strings.NewReader(payload))
	if err != nil || value.Issues[0].Code != "future.display_diagnostic" {
		t.Fatalf("open display code rejected: %#v %v", value, err)
	}
	_, err = decodeServerResponse[ServerReadinessStatusResponse](strings.NewReader(strings.Repeat(" ", maxManagementResponseBytes+1)))
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
