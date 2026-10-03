package management

import (
	"context"
	"encoding/json"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type testInstallCoordinator struct {
	registry *tasks.Registry
}

func (c testInstallCoordinator) Accept(_ context.Context, _ plugins.InstallRequest) (string, error) {
	return c.registry.Create("plugin.install", "install plugin")
}

func (testInstallCoordinator) Cancel(string) bool { return false }
func (testInstallCoordinator) Close() error       { return nil }

func trustedInstallRequest() pluginInstallRequest {
	return pluginInstallRequest{
		SourceType:           "local_zip",
		Source:               "C:/plugins/weather.zip",
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

func decodeErrorEnvelope(t fataler, body []byte) httpapi.ErrorEnvelope {
	var env httpapi.ErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("failed to decode error envelope: %v\nbody: %s", err, body)
	}
	return env
}
