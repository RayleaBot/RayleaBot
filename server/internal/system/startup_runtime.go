package system

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/system/startup"
)

var inspectStartupRuntime = func(repoRoot, kind string) (*deps.BootstrapInspection, error) {
	return deps.NewManager(repoRoot).Inspect(kind)
}

var prepareStartupRuntime = func(ctx context.Context, repoRoot, kind string) (*deps.PrepareReport, error) {
	return deps.NewManager(repoRoot).PrepareWithReport(ctx, kind)
}

var prepareStartupRuntimeWithProgress = func(ctx context.Context, repoRoot, kind string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
	if progress == nil {
		return prepareStartupRuntime(ctx, repoRoot, kind)
	}
	return deps.NewManager(repoRoot).PrepareWithReportOptions(ctx, kind, deps.PrepareOptions{Progress: progress})
}

type StartupRuntimePhase = startup.Phase
type StartupRuntimeState = startup.State

type startupRuntimePhase = StartupRuntimePhase
type startupRuntimeState = StartupRuntimeState

const (
	StartupRuntimePending     = startup.PhasePending
	StartupRuntimeReady       = startup.PhaseReady
	StartupRuntimeFailed      = startup.PhaseFailed
	StartupRuntimeNotRequired = startup.PhaseNotRequired
	startupRuntimePending     = StartupRuntimePending
	startupRuntimeReady       = StartupRuntimeReady
	startupRuntimeFailed      = StartupRuntimeFailed
	startupRuntimeNotRequired = StartupRuntimeNotRequired
)
