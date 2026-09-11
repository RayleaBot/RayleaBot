package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"pgregory.net/rapid"
)

type queueFullInstaller struct{}

func (queueFullInstaller) Accept(context.Context, plugins.InstallAcceptance) (string, error) {
	return "", tasks.ErrQueueFull
}

func (queueFullInstaller) Cancel(string) bool { return false }
func (queueFullInstaller) Close() error       { return nil }

type inspectionInstaller struct {
	inspection plugins.InstallInspection
	request    plugins.InstallRequest
}

func (installer *inspectionInstaller) Inspect(_ context.Context, request plugins.InstallRequest) (plugins.InstallInspection, error) {
	installer.request = request
	return installer.inspection, nil
}

func (*inspectionInstaller) Accept(context.Context, plugins.InstallAcceptance) (string, error) {
	return "task_inspected", nil
}

func (*inspectionInstaller) Cancel(string) bool { return false }
func (*inspectionInstaller) Close() error       { return nil }

func TestInstallInspectHandlerReturnsDigestBoundMetadata(t *testing.T) {
	installer := &inspectionInstaller{inspection: plugins.InstallInspection{
		InspectionID:   strings.Repeat("i", 64),
		ExpiresAt:      time.Date(2026, 7, 10, 12, 15, 0, 0, time.UTC),
		PackageSHA256:  strings.Repeat("a", 64),
		SourceType:     "local_zip",
		Source:         "C:/plugins/weather.zip",
		PluginID:       "example.weather",
		PluginName:     "Weather",
		Version:        "1.0.0",
		Author:         "example",
		License:        "MIT",
		SourceLabel:    "本地插件包",
		Permissions:    map[string]plugins.PermissionGrant{"http.request": {}},
		TargetPlatform: "windows-x64",
		Backend:        plugins.InstallBackendInspection{Entry: "bin/weather", Path: "bin/weather.exe", Size: 1024},
		UI:             plugins.InstallUIInspection{Enabled: true, Entry: "ui/index.html", FileCount: 3},
		Artifact:       plugins.ArtifactInspection{Valid: true, Version: "2", FileCount: 8},
	}}
	handler := newInstallInspectHandler(plugincatalog.New(nil), installer)
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/install/inspect", strings.NewReader(`{"source_type":"local_zip","source":"C:/plugins/weather.zip"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response pluginInstallInspectionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.InspectionID != installer.inspection.InspectionID || response.PackageSHA256 != installer.inspection.PackageSHA256 {
		t.Fatalf("unexpected inspection response: %#v", response)
	}
	if installer.request.Source != "C:/plugins/weather.zip" {
		t.Fatalf("unexpected inspect request: %#v", installer.request)
	}
}

func TestInstallHandlerRequiresTrustedCodeConfirmation(t *testing.T) {
	payload := trustedInstallRequest()
	payload.TrustedCodeConfirmed = false
	body, _ := json.Marshal(payload)
	handler := newInstallHandler(&inspectionInstaller{})
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", recorder.Code, recorder.Body.String())
	}
	if envelope := decodeErrorEnvelope(t, recorder.Body.Bytes()); envelope.Error.Code != "plugin.trusted_code_confirmation_required" {
		t.Fatalf("unexpected error code: %q", envelope.Error.Code)
	}
}

func TestInstallHandlerMapsQueueFullWithoutCreatingTask(t *testing.T) {
	registry := tasks.NewRegistry()
	handler := newInstallHandler(queueFullInstaller{})
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{"inspection_id":"iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii","package_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","trusted_code_confirmed":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", recorder.Code, recorder.Body.String())
	}
	if envelope := decodeErrorEnvelope(t, recorder.Body.Bytes()); envelope.Error.Code != "platform.task_queue_full" {
		t.Fatalf("error code = %q, want platform.task_queue_full", envelope.Error.Code)
	}
	if len(registry.List()) != 0 {
		t.Fatal("queue-full handler created a pending task")
	}
}

func TestInstallCreatesQueryableTask(t *testing.T) {
	router, taskRegistry := setupInstallRouter()

	reqBody, _ := json.Marshal(trustedInstallRequest())
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginTaskAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TaskID == "" {
		t.Fatal("task_id is empty")
	}

	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not found in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Fatalf("task_type = %q, want %q", snap.TaskType, "plugin.install")
	}
	if snap.Status != tasks.StatusPending {
		t.Fatalf("status = %q, want %q", snap.Status, tasks.StatusPending)
	}
}

// Feature: plugin-write-api, Property 2: 无效安装请求被拒绝
// Validates: Requirements 1.3, 1.4, 1.5
func TestProperty_InvalidInstallRequestRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		router, taskRegistry := setupInstallRouter()
		tasksBefore := len(taskRegistry.List())

		variant := rapid.IntRange(0, 2).Draw(t, "variant")
		var body string
		switch variant {
		case 0:
			body = `{"package_sha256":"` + strings.Repeat("a", 64) + `","trusted_code_confirmed":true}`
		case 1:
			body = `{"inspection_id":"` + strings.Repeat("i", 64) + `","trusted_code_confirmed":true}`
		case 2:
			body = `{"inspection_id":"` + strings.Repeat("i", 64) + `","package_sha256":"` + strings.Repeat("a", 64) + `","trusted_code_confirmed":true,"source":"legacy"}`
		}

		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		expectedStatus := http.StatusConflict
		if variant == 2 {
			expectedStatus = http.StatusBadRequest
		}
		if rec.Code != expectedStatus {
			t.Fatalf("variant=%d status = %d, want %d; body = %s", variant, rec.Code, expectedStatus, rec.Body.String())
		}

		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		expectedCode := "plugin.install_inspection_required"
		if variant == 2 {
			expectedCode = pluginCodeInvalidRequest
		}
		if env.Error.Code != expectedCode {
			t.Fatalf("error.code = %q, want %q", env.Error.Code, expectedCode)
		}

		tasksAfter := len(taskRegistry.List())
		if tasksAfter != tasksBefore {
			t.Fatalf("tasks count changed from %d to %d; no task should be created for invalid request", tasksBefore, tasksAfter)
		}
	})
}
func TestInstallHandler_ValidLocalZip(t *testing.T) {
	router, taskRegistry := setupInstallRouter()

	body, _ := json.Marshal(trustedInstallRequest())
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp pluginTaskAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TaskID == "" {
		t.Fatal("task_id is empty")
	}

	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Fatalf("task_type = %q, want plugin.install", snap.TaskType)
	}
	if snap.Status != tasks.StatusPending {
		t.Fatalf("status = %q, want pending", snap.Status)
	}
}

func TestInstallHandlerRejectsLegacyInstallScriptField(t *testing.T) {
	router, taskRegistry := setupInstallRouter()

	payload := map[string]any{
		"inspection_id": strings.Repeat("i", 64), "package_sha256": strings.Repeat("a", 64),
		"trusted_code_confirmed": true, "allow_install_scripts": true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if len(taskRegistry.List()) != 0 {
		t.Fatal("legacy request unexpectedly created an install task")
	}
}
func TestInstallHandler_MissingDigest_409(t *testing.T) {
	router, _ := setupInstallRouter()

	body, _ := json.Marshal(pluginInstallRequest{InspectionID: strings.Repeat("i", 64), TrustedCodeConfirmed: true})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "plugin.install_inspection_required" {
		t.Fatalf("error.code = %q, want plugin.install_inspection_required", env.Error.Code)
	}
}

// TestInstallHandler_MalformedJSON_400: invalid JSON body returns 400.
func TestInstallHandler_MalformedJSON_400(t *testing.T) {
	router, _ := setupInstallRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != pluginCodeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, pluginCodeInvalidRequest)
	}
}
