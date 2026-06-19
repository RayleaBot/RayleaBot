package pluginapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"pgregory.net/rapid"
)

func TestProperty_InstallCreatesQueryableTask(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sourceType := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
		source := rapid.StringMatching("[a-zA-Z0-9/_\\\\.:]{1,100}").Draw(t, "source")

		router, _, taskRegistry, _ := setupRouter(nil)

		reqBody, _ := json.Marshal(pluginInstallRequest{SourceType: sourceType, Source: source})
		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
		}

		var resp taskAcceptedResponse
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
	})
}

// Feature: plugin-write-api, Property 2: 无效安装请求被拒绝
// Validates: Requirements 1.3, 1.4, 1.5
func TestProperty_InvalidInstallRequestRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		router, _, taskRegistry, _ := setupRouter(nil)
		tasksBefore := len(taskRegistry.List())

		// Generate one of several invalid request variants.
		variant := rapid.IntRange(0, 3).Draw(t, "variant")
		var body string
		switch variant {
		case 0: // missing source_type
			src := rapid.StringMatching("[a-zA-Z0-9/_]{1,50}").Draw(t, "source")
			body = `{"source":"` + src + `"}`
		case 1: // missing source
			st := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
			body = `{"source_type":"` + st + `"}`
		case 2: // invalid source_type
			badType := rapid.StringMatching("[a-z]{3,15}").
				Filter(func(s string) bool { return s != "local_zip" && s != "local_directory" }).
				Draw(t, "badType")
			src := rapid.StringMatching("[a-zA-Z0-9/_]{1,50}").Draw(t, "source")
			b, _ := json.Marshal(pluginInstallRequest{SourceType: badType, Source: src})
			body = string(b)
		case 3: // empty source
			st := rapid.SampledFrom([]string{"local_zip", "local_directory"}).Draw(t, "sourceType")
			b, _ := json.Marshal(pluginInstallRequest{SourceType: st, Source: ""})
			body = string(b)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("variant=%d status = %d, want 400; body = %s", variant, rec.Code, rec.Body.String())
		}

		env := decodeErrorEnvelope(t, rec.Body.Bytes())
		if env.Error.Code != codeInvalidRequest {
			t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
		}

		tasksAfter := len(taskRegistry.List())
		if tasksAfter != tasksBefore {
			t.Fatalf("tasks count changed from %d to %d; no task should be created for invalid request", tasksBefore, tasksAfter)
		}
	})
}
func TestInstallHandler_ValidLocalZip(t *testing.T) {
	router, _, taskRegistry, _ := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: "C:/plugins/weather.zip"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp taskAcceptedResponse
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

func TestInstallHandler_AllowsExplicitInstallScriptAuthorization(t *testing.T) {
	router, _, taskRegistry, _ := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{
		SourceType:          "local_directory",
		Source:              "C:/plugins/weather",
		AllowInstallScripts: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rec.Code, rec.Body.String())
	}

	var resp taskAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	snap, ok := taskRegistry.Get(resp.TaskID)
	if !ok {
		t.Fatalf("task %q not in registry", resp.TaskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Fatalf("task_type = %q, want plugin.install", snap.TaskType)
	}
}
func TestInstallHandler_EmptySource_400(t *testing.T) {
	router, _, _, _ := setupRouter(nil)

	body, _ := json.Marshal(pluginInstallRequest{SourceType: "local_zip", Source: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}

// TestInstallHandler_MalformedJSON_400: invalid JSON body returns 400.
func TestInstallHandler_MalformedJSON_400(t *testing.T) {
	router, _, _, _ := setupRouter(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}
