package renderapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

var (
	renderTestPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	renderTestJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type staticRenderRunner struct{}

func (staticRenderRunner) Render(_ context.Context, doc renderbrowser.Document) ([]byte, error) {
	if doc.Output == "jpeg" {
		return append([]byte(nil), renderTestJPEGBytes...), nil
	}
	return append([]byte(nil), renderTestPNGBytes...), nil
}

func TestRenderTemplateHandlersExposePreviewWorkspaceOnly(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	listRecorder := fixture.request(http.MethodGet, "/api/system/render/templates", nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200 (%s)", listRecorder.Code, listRecorder.Body.String())
	}

	var listBody listResponse
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listBody.Items) < 2 {
		t.Fatalf("expected seeded templates, got %#v", listBody.Items)
	}
	if listBody.Items[0].Source.Type == "" {
		t.Fatalf("expected template source in list response: %#v", listBody.Items[0])
	}

	detailRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu", nil)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200 (%s)", detailRecorder.Code, detailRecorder.Body.String())
	}

	var detailEnvelope map[string]map[string]any
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detailEnvelope); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}

	templateBody := detailEnvelope["template"]
	if templateBody["input_schema_json"] == nil {
		t.Fatalf("expected input_schema_json, got %#v", templateBody)
	}
	if _, ok := templateBody["preview_data_json"]; !ok {
		t.Fatalf("expected preview_data_json field, got %#v", templateBody)
	}
	source, ok := templateBody["source"].(map[string]any)
	if !ok || source["type"] != "system" {
		t.Fatalf("expected system source, got %#v", templateBody["source"])
	}
	for _, removedField := range []string{"files", "current_revision", "last_validation", "current_revision_id"} {
		if _, ok := templateBody[removedField]; ok {
			t.Fatalf("unexpected removed detail field %q in %#v", removedField, templateBody)
		}
	}
}

func TestRenderTemplateHandlersRejectUnknownTemplate(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	recorder := fixture.request(http.MethodGet, "/api/system/render/templates/missing-template", nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("detail status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
	}

	var body map[string]map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"]["code"] != "platform.template_not_found" {
		t.Fatalf("unexpected error response: %#v", body)
	}
}

func TestRenderTemplatePreviewHTMLHandlerReturnsHTML(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	recorder := fixture.request(http.MethodPost, "/api/system/render/templates/help.menu/preview-html", map[string]any{
		"theme": "default",
		"data": map[string]any{
			"title": "同步预览",
		},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("preview html status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}

	var response previewHTMLResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode preview html response: %v", err)
	}
	if response.TemplateID != "help.menu" || response.RevisionID == "" {
		t.Fatalf("unexpected preview identity: %#v", response)
	}
	if response.Width != 960 || response.Height != 640 {
		t.Fatalf("preview dimensions = %dx%d, want 960x640", response.Width, response.Height)
	}
	if !strings.Contains(response.HTML, "同步预览") {
		t.Fatalf("preview html does not contain rendered data: %s", response.HTML)
	}
}

func TestRenderTemplatePreviewHTMLHandlerRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	recorder := fixture.request(http.MethodPost, "/api/system/render/templates/help.menu/preview-html", map[string]any{
		"theme": "default",
		"data":  []any{},
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid preview html status = %d, want 400 (%s)", recorder.Code, recorder.Body.String())
	}

	var body map[string]map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"]["code"] != "platform.invalid_request" {
		t.Fatalf("unexpected error response: %#v", body)
	}
}

func TestRenderTemplateAssetHandlerStreamsAllowedResourceAndRejectsSources(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	assetRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/asset?path=../fortune.card/assets/fortune-emblem.png", nil)
	if assetRecorder.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want 200 (%s)", assetRecorder.Code, assetRecorder.Body.String())
	}
	if len(assetRecorder.Body.Bytes()) == 0 {
		t.Fatal("expected asset bytes")
	}

	for _, path := range []string{"../outside.txt", "template.html", "missing.txt"} {
		recorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/asset?path="+path, nil)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("asset %q status = %d, want 404 (%s)", path, recorder.Code, recorder.Body.String())
		}
		var body map[string]map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error response for %q: %v", path, err)
		}
		if body["error"]["code"] != "platform.resource_missing" {
			t.Fatalf("unexpected error response for %q: %#v", path, body)
		}
	}
}

func TestRenderTemplateEditorRoutesAreRemoved(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	requests := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodGet, path: "/api/system/render/templates/help.menu/source"},
		{method: http.MethodPut, path: "/api/system/render/templates/help.menu/source", body: map[string]any{}},
		{method: http.MethodPost, path: "/api/system/render/templates/help.menu/validate", body: map[string]any{}},
		{method: http.MethodGet, path: "/api/system/render/templates/help.menu/versions"},
		{method: http.MethodPost, path: "/api/system/render/templates/help.menu/rollback", body: map[string]any{}},
	}

	for _, tc := range requests {
		recorder := fixture.request(tc.method, tc.path, tc.body)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, want 404", tc.method, tc.path, recorder.Code)
		}
	}
}

type renderHTTPFixture struct {
	router   http.Handler
	renderer *renderservice.Service
	cleanup  func()
}

func newRenderHTTPFixture(t *testing.T) renderHTTPFixture {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	root := t.TempDir()
	store, err := storage.Open(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}

	renderer, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(root, "render-output"),
		Store:              store,
		Runner:             staticRenderRunner{},
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 1 << 20,
	})
	if err != nil {
		_ = store.Close()
		t.Fatalf("create render service: %v", err)
	}
	handlers := NewHandlers(renderer)

	router := chi.NewRouter()
	router.Get("/api/system/render/templates", handlers.HandleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", handlers.HandleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", handlers.HandleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", handlers.HandleSystemRenderTemplateDetail())

	cleanup := func() {
		_ = renderer.Close()
		_ = store.Close()
	}
	t.Cleanup(cleanup)

	return renderHTTPFixture{
		router:   router,
		renderer: renderer,
		cleanup:  cleanup,
	}
}

func (f renderHTTPFixture) request(method, target string, body any) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			panic(err)
		}
	}

	request := httptest.NewRequest(method, target, &payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, request)
	return recorder
}
