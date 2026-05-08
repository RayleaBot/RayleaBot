package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestRenderTemplateHandlersExposePreviewWorkspaceOnly(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	listRecorder := fixture.request(http.MethodGet, "/api/system/render/templates", nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200 (%s)", listRecorder.Code, listRecorder.Body.String())
	}

	var listBody renderTemplateListResponse
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
	source, ok := templateBody["source"].(map[string]any)
	if !ok || source["type"] != "system" {
		t.Fatalf("expected system source, got %#v", templateBody["source"])
	}
	for _, removedField := range []string{"files", "current_revision", "last_validation", "current_revision_id"} {
		if _, ok := templateBody[removedField]; ok {
			t.Fatalf("unexpected legacy detail field %q in %#v", removedField, templateBody)
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

func TestRenderPreviewHandlerAcceptsStoredTemplateAndStreamsArtifact(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	previewRecorder := fixture.request(http.MethodPost, "/api/system/render/preview", render.Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	})
	if previewRecorder.Code != http.StatusAccepted {
		t.Fatalf("preview status = %d, want 202 (%s)", previewRecorder.Code, previewRecorder.Body.String())
	}

	var accepted taskAcceptedResponse
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("decode preview accepted response: %v", err)
	}

	task := waitTask(t, fixture.tasks, accepted.TaskID, tasks.StatusSucceeded)
	if task.Result == nil {
		t.Fatalf("expected preview task result, got %#v", task)
	}

	imageURL, _ := task.Result.Details["image_url"].(string)
	artifactID, _ := task.Result.Details["artifact_id"].(string)
	if imageURL == "" || artifactID == "" {
		t.Fatalf("expected artifact metadata, got %#v", task.Result.Details)
	}

	artifactRecorder := fixture.request(http.MethodGet, imageURL, nil)
	if artifactRecorder.Code != http.StatusOK {
		t.Fatalf("artifact status = %d, want 200 (%s)", artifactRecorder.Code, artifactRecorder.Body.String())
	}
	if got := artifactRecorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("artifact content-type = %q, want image/png", got)
	}
	if len(artifactRecorder.Body.Bytes()) == 0 {
		t.Fatal("expected artifact bytes")
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
	renderer *render.Service
	tasks    *tasks.Registry
	cleanup  func()
}

func newRenderHTTPFixture(t *testing.T) renderHTTPFixture {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	root := t.TempDir()
	store, err := storage.Open(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}

	renderer, err := render.NewService(render.Options{
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

	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, 2*time.Second)
	handlers := newRenderHTTPHandlers(renderer, executor)

	router := chi.NewRouter()
	router.Post("/api/system/render/preview", handlers.handleSystemRenderPreview())
	router.Get("/api/system/render/artifacts/{artifact_id}", handlers.handleSystemRenderArtifact())
	router.Get("/api/system/render/templates", handlers.handleSystemRenderTemplateList())
	router.Get("/api/system/render/templates/{template_id}", handlers.handleSystemRenderTemplateDetail())

	cleanup := func() {
		_ = executor.Close()
		_ = renderer.Close()
		_ = store.Close()
	}
	t.Cleanup(cleanup)

	return renderHTTPFixture{
		router:   router,
		renderer: renderer,
		tasks:    registry,
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
