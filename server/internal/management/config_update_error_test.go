package management

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

type failedConfigUpdate struct{ err error }

func (f failedConfigUpdate) CurrentConfigDocument() configruntime.Document {
	return configruntime.Document{}
}
func (f failedConfigUpdate) UpdateConfigDocument(context.Context, map[string]any) (configruntime.UpdateResult, error) {
	return configruntime.UpdateResult{}, f.err
}
func (f failedConfigUpdate) ApplyHotReloadableFields(config.Config) configruntime.ApplyEffects {
	return configruntime.NewApplyEffects()
}

func TestConfigUpdateDistinguishesInvalidInputFromPersistenceFailure(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{errors.New("invalid fixture input"), 400, errorcodes.PlatformInvalidConfig},
		{&configruntime.PersistenceError{Err: errors.New("private filesystem detail")}, 500, errorcodes.PlatformInternalError},
	} {
		handler := NewConfigHandlers(failedConfigUpdate{err: test.err})
		response := httptest.NewRecorder()
		handler.HandleConfigPut()(response, httptest.NewRequest("PUT", "/api/config", strings.NewReader("{}")))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("status=%d body=%s", response.Code, response.Body)
		}
		if strings.Contains(response.Body.String(), "private filesystem detail") {
			t.Fatal("persistence failure leaked private filesystem context")
		}
	}
}
