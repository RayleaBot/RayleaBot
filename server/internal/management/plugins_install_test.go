package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"pgregory.net/rapid"
)

type queueFullInstaller struct{}

func (queueFullInstaller) Accept(context.Context, plugins.InstallRequest) (string, error) {
	return "", tasks.ErrQueueFull
}

func (queueFullInstaller) Cancel(string) bool { return false }
func (queueFullInstaller) Close() error       { return nil }

type recordingInstaller struct {
	request plugins.InstallRequest
}

func (installer *recordingInstaller) Accept(_ context.Context, request plugins.InstallRequest) (string, error) {
	installer.request = request
	return "task_recorded", nil
}

func (*recordingInstaller) Cancel(string) bool { return false }
func (*recordingInstaller) Close() error       { return nil }

func TestInstallHandlerPassesConfirmedSourceToInstaller(t *testing.T) {
	installer := &recordingInstaller{}
	body, _ := json.Marshal(trustedInstallRequest())
	recorder := httptest.NewRecorder()

	newInstallHandler(installer).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body)))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", recorder.Code, recorder.Body.String())
	}
	if installer.request.SourceType != "local_zip" || installer.request.Source != "C:/plugins/weather.zip" || !installer.request.TrustedCodeRequired || !installer.request.TrustedCodeConfirmed {
		t.Fatalf("unexpected install request: %#v", installer.request)
	}
}

func TestInstallHandlerRequiresTrustedCodeConfirmation(t *testing.T) {
	payload := trustedInstallRequest()
	payload.TrustedCodeConfirmed = false
	body, _ := json.Marshal(payload)
	installer := &recordingInstaller{}
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	newInstallHandler(installer).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", recorder.Code, recorder.Body.String())
	}
	if envelope := decodeErrorEnvelope(t, recorder.Body.Bytes()); envelope.Error.Code != "plugin.trusted_code_confirmation_required" {
		t.Fatalf("unexpected error code: %q", envelope.Error.Code)
	}
	if installer.request.Source != "" {
		t.Fatalf("unconfirmed request reached the installer: %#v", installer.request)
	}
}

func TestInstallHandlerMapsQueueFullWithoutCreatingTask(t *testing.T) {
	registry := tasks.NewRegistry()
	body, _ := json.Marshal(trustedInstallRequest())
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	newInstallHandler(queueFullInstaller{}).ServeHTTP(recorder, request)

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
	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not found in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" || snap.Status != tasks.StatusPending {
		t.Fatalf("unexpected task snapshot: %#v", snap)
	}
}

// Feature: plugin-write-api, Property 2: 无效安装请求被拒绝
func TestProperty_InvalidInstallRequestRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		router, taskRegistry := setupInstallRouter()
		tasksBefore := len(taskRegistry.List())

		body := rapid.SampledFrom([]string{
			`{"source_type":"local_zip","trusted_code_confirmed":true}`,
			`{"source_type":"catalog","source":"official","trusted_code_confirmed":true}`,
			`{"source_type":"local_zip","source":"C:/plugins/weather.zip","trusted_code_confirmed":true,"allow_install_scripts":true}`,
			`{not valid json`,
		}).Draw(t, "body")

		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
		}
		if env := decodeErrorEnvelope(t, rec.Body.Bytes()); env.Error.Code != pluginCodeInvalidRequest {
			t.Fatalf("error.code = %q, want %q", env.Error.Code, pluginCodeInvalidRequest)
		}
		if tasksAfter := len(taskRegistry.List()); tasksAfter != tasksBefore {
			t.Fatalf("tasks count changed from %d to %d; no task should be created for invalid request", tasksBefore, tasksAfter)
		}
	})
}
