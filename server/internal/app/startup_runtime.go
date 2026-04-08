package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

var inspectStartupRuntime = func(repoRoot, kind string) (*deps.BootstrapInspection, error) {
	return deps.NewManager(repoRoot).Inspect(kind)
}

var prepareStartupRuntime = func(ctx context.Context, repoRoot, kind string) (*deps.PrepareReport, error) {
	return deps.NewManager(repoRoot).PrepareWithReport(ctx, kind)
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
	return []string{"python-runtime", "nodejs-runtime"}
}

func startupRuntimeLabel(kind string) string {
	switch kind {
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return deps.ManagedResourceLabel(kind)
	}
}

func logStartupRuntimeFailure(logger *slog.Logger, kind string, err error) {
	if logger == nil || err == nil {
		return
	}

	fields := []any{
		"component", "app",
		"resource_kind", kind,
	}

	var bootstrapErr *deps.BootstrapError
	if errors.As(err, &bootstrapErr) {
		fields = append(fields, "remediation", bootstrapErr.Remediation)
	}

	logger.Warn("startup runtime prepare skipped", append(fields, "err", err.Error())...)
}

func startupRuntimeFailureIssue(kind string, err error) recovery.CompatibilityIssue {
	issue := recovery.CompatibilityIssue{
		Code:        "platform.resource_missing",
		Severity:    "warning",
		Summary:     deps.ManagedResourceLabel(kind) + "准备失败。",
		Remediation: deps.BootstrapRemediation(kind, "", ""),
	}

	var bootstrapErr *deps.BootstrapError
	if !errors.As(err, &bootstrapErr) {
		return issue
	}

	if summary := strings.TrimSpace(bootstrapErr.Message); summary != "" {
		issue.Summary = summary
		if !strings.HasSuffix(issue.Summary, "。") {
			issue.Summary += "。"
		}
	}
	if remediation := strings.TrimSpace(bootstrapErr.Remediation); remediation != "" {
		issue.Remediation = remediation
	}
	return issue
}

func newStartupRuntimeStates(requiredKinds []string) map[string]startupRuntimeState {
	states := make(map[string]startupRuntimeState, len(startupRuntimeKinds()))
	for _, kind := range startupRuntimeKinds() {
		state := startupRuntimeState{Phase: startupRuntimeNotRequired}
		if containsRuntimeKind(requiredKinds, kind) {
			state.Phase = startupRuntimePending
		}
		states[kind] = state
	}
	return states
}

func (a *App) resetStartupRuntimeStates(requiredKinds []string) {
	if a == nil {
		return
	}
	a.startupRuntimeMu.Lock()
	defer a.startupRuntimeMu.Unlock()
	a.startupRuntimeStates = newStartupRuntimeStates(requiredKinds)
}

func (a *App) setStartupRuntimeState(kind string, phase startupRuntimePhase, issue *recovery.CompatibilityIssue) {
	if a == nil || strings.TrimSpace(kind) == "" {
		return
	}
	a.startupRuntimeMu.Lock()
	defer a.startupRuntimeMu.Unlock()
	if a.startupRuntimeStates == nil {
		a.startupRuntimeStates = newStartupRuntimeStates(nil)
	}
	var issueCopy *recovery.CompatibilityIssue
	if issue != nil {
		copied := *issue
		issueCopy = &copied
	}
	a.startupRuntimeStates[kind] = startupRuntimeState{
		Phase: phase,
		Issue: issueCopy,
	}
}

func (a *App) startupRuntimeState(kind string) (startupRuntimeState, bool) {
	if a == nil {
		return startupRuntimeState{}, false
	}
	a.startupRuntimeMu.RLock()
	defer a.startupRuntimeMu.RUnlock()
	if a.startupRuntimeStates == nil {
		return startupRuntimeState{}, false
	}
	state, ok := a.startupRuntimeStates[kind]
	return state, ok
}

func (a *App) startupRequiredRuntimeKinds() []string {
	if a == nil {
		return nil
	}
	return startupRuntimeKinds()
}

func (a *App) autoPrepareRuntimeEnvironments(ctx context.Context) {
	if a == nil || a.repoRoot == "" {
		return
	}

	requiredKinds := a.startupRequiredRuntimeKinds()
	a.resetStartupRuntimeStates(requiredKinds)
	if len(requiredKinds) == 0 {
		return
	}

	for _, kind := range requiredKinds {
		if err := ctx.Err(); err != nil {
			return
		}

		inspection, err := inspectStartupRuntime(a.repoRoot, kind)
		if err != nil {
			issue := runtimeInspectionIssue(kind, err)
			a.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(a.Logger, kind, err)
			continue
		}
		if !inspection.MetadataComplete {
			issue := runtimeMetadataIssue(kind)
			a.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			continue
		}
		if inspection.PreparedStorePresent {
			a.setStartupRuntimeState(kind, startupRuntimeReady, nil)
			continue
		}

		label := startupRuntimeLabel(kind)
		a.setStartupRuntimeState(kind, startupRuntimePending, nil)
		if a.Logger != nil {
			a.Logger.Info(
				"startup runtime prepare requested",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"cached_archive_present", inspection.CachedArchivePresent,
			)
		}

		report, err := prepareStartupRuntime(ctx, a.repoRoot, kind)
		if err != nil {
			issue := startupRuntimeFailureIssue(kind, err)
			a.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(a.Logger, kind, err)
			continue
		}

		a.setStartupRuntimeState(kind, startupRuntimeReady, nil)
		if a.Logger != nil {
			a.Logger.Info(
				"startup runtime prepare completed",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"used_cached_archive", report.UsedCachedArchive,
				"used_prepared_store", report.UsedPreparedStore,
				"store_root", report.StoreRoot,
			)
		}
	}

	a.reconcileRecoverySummaryBestEffort("startup.runtime_prepare")
}
