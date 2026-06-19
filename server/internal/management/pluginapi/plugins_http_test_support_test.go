package pluginapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type stubDesiredStateRepository struct {
	saved map[string]string
}

func (r *stubDesiredStateRepository) LoadDesiredStates(context.Context) (map[string]string, error) {
	if r == nil {
		return nil, nil
	}
	return r.saved, nil
}

func (r *stubDesiredStateRepository) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	if r.saved == nil {
		r.saved = make(map[string]string)
	}
	r.saved[pluginID] = desiredState
	return nil
}

func (r *stubDesiredStateRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func setupRouter(entries []plugins.Snapshot) (chi.Router, plugins.CatalogView, *tasks.Registry, *stubDesiredStateRepository) {
	catalog := newTestCatalog(entries)
	taskRegistry := tasks.NewRegistry()
	repo := &stubDesiredStateRepository{}
	router := chi.NewRouter()
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, nil))
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, nil, nil, nil))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, nil, nil, nil))
	return router, catalog, taskRegistry, repo
}

type stubDesiredStateController struct {
	enableResult  plugins.Snapshot
	enableErr     error
	disableResult plugins.Snapshot
	disableErr    error
	reloadResult  plugins.Snapshot
	reloadErr     error
	recoverResult plugins.Snapshot
	recoverErr    error
}

func (s *stubDesiredStateController) Enable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.enableResult, s.enableErr
}

func (s *stubDesiredStateController) Disable(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.disableResult, s.disableErr
}

func (s *stubDesiredStateController) Reload(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.reloadResult, s.reloadErr
}

func (s *stubDesiredStateController) RecoverFromDeadLetter(_ context.Context, _ string) (plugins.Snapshot, error) {
	return s.recoverResult, s.recoverErr
}

type fataler interface {
	Fatalf(format string, args ...any)
}

func decodeErrorEnvelope(t fataler, body []byte) errorEnvelope {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("failed to decode error envelope: %v\nbody: %s", err, body)
	}
	return env
}
