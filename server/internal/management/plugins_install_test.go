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
)

type queueFullInstaller struct{}

func (queueFullInstaller) Accept(context.Context, plugins.InstallRequest) (string, error) {
	return "", tasks.ErrQueueFull
}

func (queueFullInstaller) Cancel(string) bool { return false }
func (queueFullInstaller) Close() error       { return nil }

type recordingInstaller struct {
	accepted int
	request  plugins.InstallRequest
}

func (installer *recordingInstaller) Accept(_ context.Context, request plugins.InstallRequest) (string, error) {
	installer.accepted++
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

func TestInstallHandlerMapsQueueFull(t *testing.T) {
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
}

func TestInstallHandlerRejectsInvalidRequestBeforeInstaller(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing source", body: `{"source_type":"local_zip","trusted_code_confirmed":true}`},
		{name: "unsupported source type", body: `{"source_type":"catalog","source":"official","trusted_code_confirmed":true}`},
		{name: "unknown field", body: `{"source_type":"local_zip","source":"C:/plugins/weather.zip","trusted_code_confirmed":true,"allow_install_scripts":true}`},
		{name: "malformed json", body: `{not valid json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installer := &recordingInstaller{}
			request := httptest.NewRequest(http.MethodPost, "/api/plugins/install", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			newInstallHandler(installer).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", recorder.Code, recorder.Body.String())
			}
			if envelope := decodeErrorEnvelope(t, recorder.Body.Bytes()); envelope.Error.Code != pluginCodeInvalidRequest {
				t.Fatalf("error code = %q, want %q", envelope.Error.Code, pluginCodeInvalidRequest)
			}
			if installer.accepted != 0 {
				t.Fatalf("invalid request reached the installer: %#v", installer.request)
			}
		})
	}
}
