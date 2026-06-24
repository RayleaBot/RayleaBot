package app

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func newAppRuntimeState(buildState appBuildState) *appRuntimeState {
	state := buildState.core
	return &state
}

func (s *appRuntimeState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}

func (s *appRuntimeState) CurrentSummary() config.Summary {
	if s == nil {
		return config.Summary{}
	}
	return s.Summary
}

func (s *appRuntimeState) SetConfig(cfg config.Config) {
	if s != nil {
		s.Config = cfg
	}
}

func (s *appRuntimeState) SetSummary(summary config.Summary) {
	if s != nil {
		s.Summary = summary
	}
}

func (s *appRuntimeState) RuntimeLogger() *slog.Logger {
	if s == nil {
		return nil
	}
	return s.Logger
}

func (s *appRuntimeState) RuntimeLogLevel() *logging.LevelController {
	if s == nil {
		return nil
	}
	return s.LogLevel
}

func (s *appRuntimeState) RepoRoot() string {
	if s == nil {
		return ""
	}
	return s.repoRoot
}

func (s *appRuntimeState) StartedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.startedAt
}

func (s *appRuntimeState) RedactString(value string) string {
	return s.redactString(value)
}

func (s *appRuntimeState) AddRedactionValues(values ...string) {
	if s == nil || s.addRedactionValues == nil {
		return
	}
	s.addRedactionValues(values...)
}
