package app

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

type startupRuntimePhase string

const (
	startupRuntimePending     startupRuntimePhase = "pending"
	startupRuntimeReady       startupRuntimePhase = "ready"
	startupRuntimeFailed      startupRuntimePhase = "failed"
	startupRuntimeNotRequired startupRuntimePhase = "not_required"
)

type startupRuntimeState struct {
	Phase startupRuntimePhase
	Issue *recovery.CompatibilityIssue
}

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
