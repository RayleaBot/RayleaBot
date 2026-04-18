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

func TestRenderTemplateHandlersSupportSaveRollbackAndHistory(t *testing.T) {
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

	sourceRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/source", nil)
	if sourceRecorder.Code != http.StatusOK {
		t.Fatalf("source status = %d, want 200 (%s)", sourceRecorder.Code, sourceRecorder.Body.String())
	}

	var sourceBody renderTemplateSourceResponse
	if err := json.Unmarshal(sourceRecorder.Body.Bytes(), &sourceBody); err != nil {
		t.Fatalf("decode source response: %v", err)
	}
	baseRevisionID := sourceBody.RevisionID
	updatedSource := sourceBody.Source
	updatedSource.HTML = `<section class="saved">{{ .title }}</section>`

	saveRecorder := fixture.request(http.MethodPut, "/api/system/render/templates/help.menu/source", renderTemplateSourceUpdateRequest{
		BaseRevisionID: baseRevisionID,
		Source:         updatedSource,
		Message:        "调整帮助卡片",
	})
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200 (%s)", saveRecorder.Code, saveRecorder.Body.String())
	}

	var saveBody renderTemplateDetailResponse
	if err := json.Unmarshal(saveRecorder.Body.Bytes(), &saveBody); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if saveBody.Template.CurrentRevision.RevisionID == baseRevisionID {
		t.Fatalf("expected save to create a new revision")
	}

	rollbackRecorder := fixture.request(http.MethodPost, "/api/system/render/templates/help.menu/rollback", renderTemplateRollbackRequest{
		TargetRevisionID: baseRevisionID,
		BaseRevisionID:   saveBody.Template.CurrentRevision.RevisionID,
		Message:          "恢复到初始版本",
	})
	if rollbackRecorder.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200 (%s)", rollbackRecorder.Code, rollbackRecorder.Body.String())
	}

	var rollbackBody renderTemplateDetailResponse
	if err := json.Unmarshal(rollbackRecorder.Body.Bytes(), &rollbackBody); err != nil {
		t.Fatalf("decode rollback response: %v", err)
	}
	if rollbackBody.Template.CurrentRevision.Kind != "rollback" {
		t.Fatalf("unexpected rollback revision kind: %#v", rollbackBody.Template.CurrentRevision)
	}

	versionsRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/versions", nil)
	if versionsRecorder.Code != http.StatusOK {
		t.Fatalf("versions status = %d, want 200 (%s)", versionsRecorder.Code, versionsRecorder.Body.String())
	}

	var versionsBody renderTemplateVersionListResponse
	if err := json.Unmarshal(versionsRecorder.Body.Bytes(), &versionsBody); err != nil {
		t.Fatalf("decode versions response: %v", err)
	}
	if len(versionsBody.Items) < 3 {
		t.Fatalf("expected save and rollback revisions, got %#v", versionsBody.Items)
	}
	if versionsBody.Items[0].Kind != "rollback" {
		t.Fatalf("expected newest revision to be rollback, got %#v", versionsBody.Items[0])
	}
}

func TestRenderTemplateHandlersValidateInvalidSourceAndDetectRevisionConflict(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	sourceRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/source", nil)
	if sourceRecorder.Code != http.StatusOK {
		t.Fatalf("source status = %d, want 200 (%s)", sourceRecorder.Code, sourceRecorder.Body.String())
	}

	var sourceBody renderTemplateSourceResponse
	if err := json.Unmarshal(sourceRecorder.Body.Bytes(), &sourceBody); err != nil {
		t.Fatalf("decode source response: %v", err)
	}

	validateRecorder := fixture.request(http.MethodPost, "/api/system/render/templates/help.menu/validate", renderTemplateValidateRequest{
		Source: &render.TemplateSource{
			ManifestJSON: map[string]any{
				"version": "1",
			},
			HTML:            "<section></section>",
			Stylesheet:      ".menu-card {}",
			InputSchemaJSON: nil,
		},
	})
	if validateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("validate status = %d, want 400 (%s)", validateRecorder.Code, validateRecorder.Body.String())
	}

	var validateError map[string]map[string]any
	if err := json.Unmarshal(validateRecorder.Body.Bytes(), &validateError); err != nil {
		t.Fatalf("decode validate error: %v", err)
	}
	if validateError["error"]["code"] != "platform.template_source_invalid" {
		t.Fatalf("unexpected validate error: %#v", validateError)
	}

	updatedSource := sourceBody.Source
	updatedSource.HTML = `<section class="saved">{{ .title }}</section>`
	saveRecorder := fixture.request(http.MethodPut, "/api/system/render/templates/help.menu/source", renderTemplateSourceUpdateRequest{
		BaseRevisionID: sourceBody.RevisionID,
		Source:         updatedSource,
		Message:        "保存模板修改",
	})
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200 (%s)", saveRecorder.Code, saveRecorder.Body.String())
	}

	conflictRecorder := fixture.request(http.MethodPut, "/api/system/render/templates/help.menu/source", renderTemplateSourceUpdateRequest{
		BaseRevisionID: sourceBody.RevisionID,
		Source:         updatedSource,
		Message:        "重复保存旧版本",
	})
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, want 409 (%s)", conflictRecorder.Code, conflictRecorder.Body.String())
	}

	var conflictError map[string]map[string]any
	if err := json.Unmarshal(conflictRecorder.Body.Bytes(), &conflictError); err != nil {
		t.Fatalf("decode conflict error: %v", err)
	}
	if conflictError["error"]["code"] != "platform.template_revision_conflict" {
		t.Fatalf("unexpected conflict error: %#v", conflictError)
	}
}

func TestRenderPreviewHandlerAcceptsDraftSourceWithoutPersistingRevision(t *testing.T) {
	t.Parallel()

	fixture := newRenderHTTPFixture(t)

	sourceRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/source", nil)
	if sourceRecorder.Code != http.StatusOK {
		t.Fatalf("source status = %d, want 200 (%s)", sourceRecorder.Code, sourceRecorder.Body.String())
	}

	var sourceBody renderTemplateSourceResponse
	if err := json.Unmarshal(sourceRecorder.Body.Bytes(), &sourceBody); err != nil {
		t.Fatalf("decode source response: %v", err)
	}

	draftSource := sourceBody.Source
	draftSource.HTML = `<section class="draft">{{ .title }}</section>`

	previewRecorder := fixture.request(http.MethodPost, "/api/system/render/preview", render.Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
		Draft: &render.TemplateDraft{Source: draftSource},
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
	if task.Result.Details["template"] != "help.menu" {
		t.Fatalf("unexpected preview task details: %#v", task.Result.Details)
	}

	sourceAfterRecorder := fixture.request(http.MethodGet, "/api/system/render/templates/help.menu/source", nil)
	if sourceAfterRecorder.Code != http.StatusOK {
		t.Fatalf("source after preview status = %d, want 200 (%s)", sourceAfterRecorder.Code, sourceAfterRecorder.Body.String())
	}

	var sourceAfter renderTemplateSourceResponse
	if err := json.Unmarshal(sourceAfterRecorder.Body.Bytes(), &sourceAfter); err != nil {
		t.Fatalf("decode source after preview response: %v", err)
	}
	if sourceAfter.RevisionID != sourceBody.RevisionID {
		t.Fatalf("draft preview changed current revision: got %q want %q", sourceAfter.RevisionID, sourceBody.RevisionID)
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
	router.Get("/api/system/render/templates", handlers.handleSystemRenderTemplateList())
	router.Get("/api/system/render/templates/{template_id}", handlers.handleSystemRenderTemplateDetail())
	router.Get("/api/system/render/templates/{template_id}/source", handlers.handleSystemRenderTemplateSource())
	router.Put("/api/system/render/templates/{template_id}/source", handlers.handleSystemRenderTemplateSourcePut())
	router.Post("/api/system/render/templates/{template_id}/validate", handlers.handleSystemRenderTemplateValidate())
	router.Get("/api/system/render/templates/{template_id}/versions", handlers.handleSystemRenderTemplateVersions())
	router.Post("/api/system/render/templates/{template_id}/rollback", handlers.handleSystemRenderTemplateRollback())

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
