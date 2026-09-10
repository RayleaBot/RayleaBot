package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

type failingConfigPersistence struct{ ConfigService }

func (failingConfigPersistence) UpdateConfigDocument(context.Context, map[string]any) (configruntime.UpdateResult, error) {
	return configruntime.UpdateResult{}, &configruntime.PersistenceError{Err: errors.New("private fixture persistence detail")}
}

func TestConfigPersistenceFailureIsServerErrorWithoutPrivateDetails(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader("{}"))
	NewConfigHandlers(failingConfigPersistence{}).HandleConfigPut().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != errorcodes.PlatformInternalError || strings.Contains(response.Body.String(), "private fixture") {
		t.Fatalf("unexpected persistence error response: %s", response.Body.String())
	}
}
