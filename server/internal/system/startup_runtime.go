package system

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

var inspectStartupRuntime = func(repoRoot, kind string) (*deps.BootstrapInspection, error) {
	return deps.NewDiagnostics(repoRoot).InspectRuntime(kind)
}

var prepareStartupRuntime = func(ctx context.Context, repoRoot, kind string) (*deps.PrepareReport, error) {
	return deps.NewRuntime(repoRoot).PrepareWithReport(ctx, kind)
}

var prepareStartupRuntimeWithProgress = func(ctx context.Context, repoRoot, kind string, progress deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
	if progress == nil {
		return prepareStartupRuntime(ctx, repoRoot, kind)
	}
	return deps.NewRuntime(repoRoot).PrepareWithReportOptions(ctx, kind, deps.PrepareOptions{Progress: progress})
}

type startupRuntimePhase = StartupRuntimePhase
type startupRuntimeState = StartupRuntimeState

const (
	StartupRuntimePending     = StartupRuntimePhasePending
	StartupRuntimeReady       = StartupRuntimePhaseReady
	StartupRuntimeFailed      = StartupRuntimePhaseFailed
	StartupRuntimeNotRequired = StartupRuntimePhaseNotRequired
	startupRuntimePending     = StartupRuntimePending
	startupRuntimeReady       = StartupRuntimeReady
	startupRuntimeFailed      = StartupRuntimeFailed
	startupRuntimeNotRequired = StartupRuntimeNotRequired
)
