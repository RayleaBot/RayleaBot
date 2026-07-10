package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

type updateServiceStub struct {
	status releaseupdate.StatusSnapshot
	check  releaseupdate.StatusSnapshot
	err    error
}

func (stub *updateServiceStub) Status() releaseupdate.StatusSnapshot { return stub.status }

func (stub *updateServiceStub) Check(context.Context) (releaseupdate.StatusSnapshot, error) {
	return stub.check, stub.err
}

func TestUpdateStatusHandlerReturnsSharedState(t *testing.T) {
	checkedAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	handler := NewUpdateHandlers(&updateServiceStub{status: releaseupdate.StatusSnapshot{
		State:            "update_available",
		CurrentVersion:   "1.0.0",
		AvailableVersion: "1.1.0",
		CheckedAt:        &checkedAt,
		UpdateMode:       "guided",
		ReleaseNotesRef:  "https://example.com/releases/v1.1.0",
	}}).HandleStatus()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/update/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d", recorder.Code)
	}
	var response updateStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.State != "update_available" || response.AvailableVersion != "1.1.0" || response.CheckedAt == nil {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestUpdateCheckHandlerRejectsConcurrentCheck(t *testing.T) {
	handler := NewUpdateHandlers(&updateServiceStub{err: releaseupdate.ErrCheckInProgress}).HandleCheck()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/update/check", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUpdateCheckHandlerDoesNotLeakInternalFailure(t *testing.T) {
	handler := NewUpdateHandlers(&updateServiceStub{err: errors.New("private upstream details")}).HandleCheck()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/update/check", nil))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status code = %d", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" || contains(body, "private upstream details") {
		t.Fatalf("unsafe error response: %s", body)
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
