package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"rayleabot/server/internal/app"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/render"
	"rayleabot/server/internal/tasks"
)

var testPreviewPNGBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")

type stubRenderRunner struct{}

func (stubRenderRunner) Render(context.Context, render.Document) ([]byte, error) {
	return append([]byte(nil), testPreviewPNGBytes...), nil
}

func TestSystemRenderPreviewAcceptsTaskAndExposesArtifact(t *testing.T) {
	t.Parallel()

	application := newRenderReadyTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.system-render-preview-accepted.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(fixture.Request.Method, server.URL+fixture.Request.Path, encodeBodyReader(t, fixture.Request.Body))
	if err != nil {
		t.Fatalf("create render preview request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform render preview request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected render preview status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("expected task_id, got %#v", body)
	}

	snapshot := waitForTaskStatus(t, application.Tasks, taskID, tasks.StatusSucceeded)
	if snapshot.TaskType != "render.preview" {
		t.Fatalf("unexpected task type: got %q want %q", snapshot.TaskType, "render.preview")
	}
	if snapshot.Result == nil || snapshot.Result.Details == nil {
		t.Fatalf("expected preview task result details, got %#v", snapshot)
	}

	imageURL, _ := snapshot.Result.Details["image_url"].(string)
	if imageURL == "" {
		t.Fatalf("expected image_url in task result details, got %#v", snapshot.Result.Details)
	}

	artifactRequest, err := http.NewRequest(http.MethodGet, server.URL+imageURL, nil)
	if err != nil {
		t.Fatalf("create render artifact request: %v", err)
	}
	artifactRequest.Header.Set("Authorization", "Bearer "+token)

	artifactResponse, err := server.Client().Do(artifactRequest)
	if err != nil {
		t.Fatalf("perform render artifact request: %v", err)
	}
	defer artifactResponse.Body.Close()
	if artifactResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected render artifact status: got %d want %d", artifactResponse.StatusCode, http.StatusOK)
	}
	if artifactResponse.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected render artifact content-type: %q", artifactResponse.Header.Get("Content-Type"))
	}
	if len(readAll(t, artifactResponse)) == 0 {
		t.Fatal("expected non-empty render artifact body")
	}
}

func TestSystemRenderPreviewRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	application := newRenderReadyTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "invalid.system-render-preview-invalid.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(fixture.Request.Method, server.URL+fixture.Request.Path, encodeBodyReader(t, fixture.Request.Body))
	if err != nil {
		t.Fatalf("create invalid render preview request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform invalid render preview request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected invalid render preview status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	assertErrorEnvelopeMatchesFixture(t, body, fixture.Response.Body, "platform.invalid_request")
}

func newRenderReadyTestApp(t *testing.T, authOptions ...auth.Option) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))
	configPath := writeYAMLConfig(t, fixture.Input)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := app.New(app.Options{
		ConfigPath:   configPath,
		SchemaPath:   schemaPath,
		AuthOptions:  authOptions,
		RenderRunner: stubRenderRunner{},
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application
}
