package startup

import (
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type Phase string

const (
	PhasePending     Phase = "pending"
	PhaseReady       Phase = "ready"
	PhaseFailed      Phase = "failed"
	PhaseNotRequired Phase = "not_required"
)

type State struct {
	Phase Phase
	Issue *recovery.CompatibilityIssue
}

func Kinds() []string {
	return []string{"chromium", "python-runtime", "nodejs-runtime"}
}

func ManagedDiagnosticKinds() []string {
	return []string{"python-runtime", "nodejs-runtime"}
}

func Label(kind string) string {
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
