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

func logStartupRuntimeProgress(logger *slog.Logger, event deps.PrepareProgress) {
	if logger == nil {
		return
	}
	fields := []any{
		"component", "runtime_prepare",
		"resource_kind", event.Kind,
		"label", event.Label,
		"stage", event.Stage,
		"status", event.Status,
	}
	if event.ResourceID != "" {
		fields = append(fields, "resource_id", event.ResourceID)
	}
	if event.Version != "" {
		fields = append(fields, "version", event.Version)
	}
	if event.SourceLabel != "" {
		fields = append(fields, "source_label", event.SourceLabel)
	}
	if event.SourceURL != "" {
		fields = append(fields, "source_url", event.SourceURL)
	}
	if event.ArchivePath != "" {
		fields = append(fields, "archive_path", event.ArchivePath)
	}
	if event.StoreRoot != "" {
		fields = append(fields, "store_root", event.StoreRoot)
	}
	if event.Progress > 0 || event.Status == "succeeded" {
		fields = append(fields, "progress", event.Progress)
	}
	if event.DownloadedBytes > 0 {
		fields = append(fields, "downloaded_bytes", event.DownloadedBytes)
	}
	if event.TotalBytes > 0 {
		fields = append(fields, "total_bytes", event.TotalBytes)
	}
	if event.ExtractedEntries > 0 {
		fields = append(fields, "extracted_entries", event.ExtractedEntries)
	}
	if event.TotalEntries > 0 {
		fields = append(fields, "total_entries", event.TotalEntries)
	}
	if event.Summary != "" {
		fields = append(fields, "summary", event.Summary)
	}
	if event.Error != "" {
		fields = append(fields, "err", event.Error)
	}
	if event.Status == "failed" {
		logger.Warn("runtime_prepare_progress", fields...)
		return
	}
	logger.Info("runtime_prepare_progress", fields...)
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

func (s *systemService) resetStartupRuntimeStates(requiredKinds []string) {
	if s == nil {
		return
	}
	s.state.startupRuntimeMu.Lock()
	defer s.state.startupRuntimeMu.Unlock()
	s.state.startupRuntimeStates = newStartupRuntimeStates(requiredKinds)
}

func (s *systemService) setStartupRuntimeState(kind string, phase startupRuntimePhase, issue *recovery.CompatibilityIssue) {
	if s == nil || strings.TrimSpace(kind) == "" {
		return
	}
	s.state.startupRuntimeMu.Lock()
	defer s.state.startupRuntimeMu.Unlock()
	if s.state.startupRuntimeStates == nil {
		s.state.startupRuntimeStates = newStartupRuntimeStates(nil)
	}
	var issueCopy *recovery.CompatibilityIssue
	if issue != nil {
		copied := *issue
		issueCopy = &copied
	}
	s.state.startupRuntimeStates[kind] = startupRuntimeState{
		Phase: phase,
		Issue: issueCopy,
	}
}

func (s *systemService) startupRuntimeState(kind string) (startupRuntimeState, bool) {
	if s == nil {
		return startupRuntimeState{}, false
	}
	s.state.startupRuntimeMu.RLock()
	defer s.state.startupRuntimeMu.RUnlock()
	if s.state.startupRuntimeStates == nil {
		return startupRuntimeState{}, false
	}
	state, ok := s.state.startupRuntimeStates[kind]
	return state, ok
}

func (s *systemService) startupRequiredRuntimeKinds() []string {
	if s == nil {
		return nil
	}
	kinds := make([]string, 0, len(startupRuntimeKinds()))
	if strings.TrimSpace(s.state.Config.Render.BrowserPath) == "" {
		kinds = append(kinds, "chromium")
	}
	kinds = append(kinds, "python-runtime", "nodejs-runtime")
	return kinds
}

func (s *systemService) autoPrepareRuntimeEnvironments(ctx context.Context) {
	if s == nil || s.state.repoRoot == "" {
		return
	}

	requiredKinds := s.startupRequiredRuntimeKinds()
	s.resetStartupRuntimeStates(requiredKinds)
	if len(requiredKinds) == 0 {
		return
	}

	for _, kind := range requiredKinds {
		if err := ctx.Err(); err != nil {
			return
		}

		inspection, err := inspectStartupRuntime(s.state.repoRoot, kind)
		if err != nil {
			issue := runtimeInspectionIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(s.state.Logger, kind, err)
			continue
		}
		if !inspection.MetadataComplete {
			issue := runtimeMetadataIssue(kind)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			continue
		}
		if inspection.PreparedStorePresent {
			s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
			continue
		}

		label := startupRuntimeLabel(kind)
		s.setStartupRuntimeState(kind, startupRuntimePending, nil)
		if s.state.Logger != nil {
			s.state.Logger.Info(
				"startup runtime prepare requested",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"cached_archive_present", inspection.CachedArchivePresent,
			)
		}

		report, err := prepareStartupRuntimeWithProgress(ctx, s.state.repoRoot, kind, func(event deps.PrepareProgress) {
			logStartupRuntimeProgress(s.state.Logger, event)
		})
		if err != nil {
			issue := startupRuntimeFailureIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(s.state.Logger, kind, err)
			continue
		}

		s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
		if kind == "chromium" && s.renderer != nil && report.PreparedEntrypoint != "" {
			s.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
		}
		if s.state.Logger != nil {
			s.state.Logger.Info(
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

	s.ReconcileRecoverySummaryBestEffort("startup.runtime_prepare")
}
