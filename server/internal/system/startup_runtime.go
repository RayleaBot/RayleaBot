package system

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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

type StartupRuntimePhase string

const (
	StartupRuntimePending     StartupRuntimePhase = "pending"
	StartupRuntimeReady       StartupRuntimePhase = "ready"
	StartupRuntimeFailed      StartupRuntimePhase = "failed"
	StartupRuntimeNotRequired StartupRuntimePhase = "not_required"
)

type StartupRuntimeState struct {
	Phase StartupRuntimePhase
	Issue *recovery.CompatibilityIssue
}

type startupRuntimePhase = StartupRuntimePhase
type startupRuntimeState = StartupRuntimeState

const (
	startupRuntimePending     = StartupRuntimePending
	startupRuntimeReady       = StartupRuntimeReady
	startupRuntimeFailed      = StartupRuntimeFailed
	startupRuntimeNotRequired = StartupRuntimeNotRequired
)

func startupRuntimeKinds() []string {
	return []string{"chromium", "python-runtime", "nodejs-runtime"}
}

func startupManagedRuntimeDiagnosticKinds() []string {
	return []string{"python-runtime", "nodejs-runtime"}
}

func startupRuntimeLabel(kind string) string {
	switch kind {
	case "chromium":
		return "Chromium 浏览环境"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return deps.ManagedResourceLabel(kind)
	}
}
