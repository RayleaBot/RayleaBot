package system

import (
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type StartupRuntimePhase string

const (
	StartupRuntimePhasePending     StartupRuntimePhase = "pending"
	StartupRuntimePhaseReady       StartupRuntimePhase = "ready"
	StartupRuntimePhaseFailed      StartupRuntimePhase = "failed"
	StartupRuntimePhaseNotRequired StartupRuntimePhase = "not_required"
)

type StartupRuntimeState struct {
	Phase StartupRuntimePhase
	Issue *recovery.CompatibilityIssue
}

func startupRuntimeKinds() []string {
	return []string{"chromium", "python-runtime", "nodejs-runtime"}
}

func managedDiagnosticKinds() []string {
	return []string{"python-runtime", "nodejs-runtime"}
}

func managedRuntimeLabel(kind string) string {
	switch kind {
	case "chromium":
		return "图片渲染 Chromium"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return deps.ManagedResourceLabel(kind)
	}
}
