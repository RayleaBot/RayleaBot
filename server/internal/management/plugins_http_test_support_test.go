package management

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type testInstallCoordinator struct {
	registry *tasks.Registry
}

func (c testInstallCoordinator) Accept(_ context.Context, _ plugins.InstallAcceptance) (string, error) {
	return c.registry.Create("plugin.install", "install inspected plugin")
}

func (testInstallCoordinator) Cancel(string) bool { return false }
func (testInstallCoordinator) Close() error       { return nil }

func setupInstallRouter() (chi.Router, *tasks.Registry) {
	registry := tasks.NewRegistry()
	router := chi.NewRouter()
	router.Post("/api/plugins/install", newInstallHandler(testInstallCoordinator{registry: registry}))
	return router, registry
}

func trustedInstallRequest() pluginInstallRequest {
	return pluginInstallRequest{
		InspectionID:         strings.Repeat("i", 64),
		PackageSHA256:        strings.Repeat("a", 64),
		TrustedCodeConfirmed: true,
	}
}

type stubDesiredStateController struct {
	calls         []string
	enableResult  plugins.Snapshot
	enableErr     error
	disableResult plugins.Snapshot
	disableErr    error
	reloadResult  plugins.Snapshot
	reloadErr     error
	recoverResult plugins.Snapshot
	recoverErr    error
}

func (s *stubDesiredStateController) Enable(_ context.Context, pluginID string) (plugins.Snapshot, error) {
	s.calls = append(s.calls, "enable:"+pluginID)
	return s.enableResult, s.enableErr
}

func (s *stubDesiredStateController) Disable(_ context.Context, pluginID string) (plugins.Snapshot, error) {
	s.calls = append(s.calls, "disable:"+pluginID)
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
