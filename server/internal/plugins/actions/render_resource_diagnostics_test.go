package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestResourceDiagnosticKeepsHostAndCategoryWithoutURLCredentials(t *testing.T) {
	var output bytes.Buffer
	cause := describeResourceFailure("https://user:fixture-private@cdn.example.invalid/a?token=fixture-query", 0, &url.Error{Op: "Get", URL: "https://cdn.example.invalid/?token=fixture-query", Err: &net.DNSError{Name: "cdn.example.invalid", Err: "fixture"}})
	logRenderImageResourceUnavailable(Deps{Logger: slog.New(slog.NewJSONHandler(&output, nil))}, ActionRequest{PluginID: "fixture"}, "media-1", "network", cause, 12*time.Millisecond)
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["resource_host"] != "cdn.example.invalid" || record["error_kind"] != "dns" || record["duration_ms"] != float64(12) {
		t.Fatalf("diagnostic=%v", record)
	}
	if strings.Contains(output.String(), "fixture-private") || strings.Contains(output.String(), "fixture-query") {
		t.Fatal("resource credentials escaped")
	}
}

func TestUnavailableResourceKeepsTemplateFallbackAndHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	var output bytes.Buffer
	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{RenderResourceRoot: t.TempDir(), Logger: slog.New(slog.NewJSONHandler(&output, nil))}, ActionRequest{PluginID: "fixture", Action: plugins.Action{RenderResources: []plugins.RenderImageResource{{ID: "media-1", URL: server.URL + "/image?token=fixture-private"}}}})
	defer cleanup()
	if err != nil || len(resources) != 0 {
		t.Fatalf("resources=%v err=%v", resources, err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["http_status"] != float64(403) || record["reason"] != "http_status" || strings.Contains(output.String(), "fixture-private") {
		t.Fatalf("diagnostic=%v", record)
	}
}

func TestSlowResourceLeavesBudgetForFallbackRendering(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	renderer := &budgetTestRenderer{}
	result, err := executeRenderImage(t.Context(), Deps{Renderer: renderer, RenderResourceRoot: t.TempDir()}, ActionRequest{PluginID: "fixture", Action: plugins.Action{
		RenderTemplate: "fixture", RenderTimeoutMS: 1500, RenderResources: []plugins.RenderImageResource{{ID: "media", URL: server.URL}},
	}})
	if err != nil || result["image_path"] != "fixture.png" || len(renderer.deadlines) != 2 {
		t.Fatalf("fallback result=%v err=%v", result, err)
	}
}
