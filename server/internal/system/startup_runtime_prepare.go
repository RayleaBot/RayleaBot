package system

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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

func newStartupRuntimeStates(requiredKinds []string) map[string]StartupRuntimeState {
	states := make(map[string]StartupRuntimeState, len(startupRuntimeKinds()))
	for _, kind := range startupRuntimeKinds() {
		state := StartupRuntimeState{Phase: StartupRuntimePhaseNotRequired}
		if containsRuntimeKind(requiredKinds, kind) {
			state.Phase = StartupRuntimePhasePending
		}
		states[kind] = state
	}
	return states
}

func (s *Service) resetStartupRuntimeStates(requiredKinds []string) {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	s.startupRuntimes = newStartupRuntimeStates(requiredKinds)
}

func (s *Service) setStartupRuntimeState(kind string, phase StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	if strings.TrimSpace(kind) == "" {
		return
	}
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	if s.startupRuntimes == nil {
		s.startupRuntimes = newStartupRuntimeStates(nil)
	}
	var issueCopy *recovery.CompatibilityIssue
	if issue != nil {
		copied := *issue
		issueCopy = &copied
	}
	s.startupRuntimes[kind] = StartupRuntimeState{
		Phase: phase,
		Issue: issueCopy,
	}
}

func (s *Service) startupRuntimeState(kind string) (StartupRuntimeState, bool) {
	s.startupMu.RLock()
	defer s.startupMu.RUnlock()
	if s.startupRuntimes == nil {
		return StartupRuntimeState{}, false
	}
	state, ok := s.startupRuntimes[kind]
	return state, ok
}

func (s *Service) StartupRuntimeState(kind string) (StartupRuntimeState, bool) {
	return s.startupRuntimeState(kind)
}

func (s *Service) SetStartupRuntimeState(kind string, phase StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	s.setStartupRuntimeState(kind, phase, issue)
}

func (s *Service) startupRequiredRuntimeKinds() []string {
	kinds := make([]string, 0, len(startupRuntimeKinds()))
	if strings.TrimSpace(s.config().Render.BrowserPath) == "" {
		kinds = append(kinds, "chromium")
	}
	kinds = append(kinds, "python-runtime", "nodejs-runtime")
	return kinds
}

func startupInspectionIssue(_ string, err error) recovery.CompatibilityIssue {
	var bootstrapErr *deps.BootstrapError
	if errors.As(err, &bootstrapErr) && (errors.Is(bootstrapErr.Err, os.ErrNotExist) || !strings.Contains(strings.ToLower(bootstrapErr.Err.Error()), "does not include")) {
		return recovery.CompatibilityIssue{
			Code:        "deps.manifest_missing",
			Severity:    "warning",
			Summary:     "运行环境清单缺失或无效。",
			Remediation: "请恢复有效的 .deps/manifest.json。",
		}
	}
	return recovery.CompatibilityIssue{
		Code:        "deps.manifest_platform_missing",
		Severity:    "warning",
		Summary:     "运行环境清单缺少当前平台资源。",
		Remediation: "请恢复当前平台的 .deps 资源清单。",
	}
}

func startupMetadataIssue(kind string) recovery.CompatibilityIssue {
	switch kind {
	case "python-runtime":
		return recovery.CompatibilityIssue{
			Code:        "deps.python_runtime_metadata_incomplete",
			Severity:    "warning",
			Summary:     "Python 运行环境元数据不完整。",
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	case "nodejs-runtime":
		return recovery.CompatibilityIssue{
			Code:        "deps.nodejs_runtime_metadata_incomplete",
			Severity:    "warning",
			Summary:     "Node.js / npm 环境元数据不完整。",
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	default:
		return recovery.CompatibilityIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "运行环境元数据不完整。",
			Remediation: "请补齐当前平台运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	}
}

func startupFailureIssue(kind string, err error) recovery.CompatibilityIssue {
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

func (s *Service) autoPrepareRuntimeEnvironments(ctx context.Context) {
	if s.repoRootPath() == "" {
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

		inspection, err := inspectStartupRuntime(s.repoRootPath(), kind)
		if err != nil {
			issue := startupInspectionIssue(kind, err)
			s.setStartupRuntimeState(kind, StartupRuntimePhaseFailed, &issue)
			logStartupFailure(s.currentLogger(), s.repoRootPath(), kind, err)
			continue
		}
		if !inspection.MetadataComplete {
			issue := startupMetadataIssue(kind)
			s.setStartupRuntimeState(kind, StartupRuntimePhaseFailed, &issue)
			continue
		}
		if inspection.PreparedStorePresent {
			s.setStartupRuntimeState(kind, StartupRuntimePhaseReady, nil)
			if kind == "chromium" && s.renderer != nil && strings.TrimSpace(inspection.SystemBrowserPath) != "" {
				s.renderer.RefreshBrowserPath(inspection.SystemBrowserPath)
			}
			continue
		}

		label := managedRuntimeLabel(kind)
		s.setStartupRuntimeState(kind, StartupRuntimePhasePending, nil)
		if s.currentLogger() != nil {
			s.currentLogger().Info(
				"运行环境准备已请求："+label,
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"cached_archive_present", inspection.CachedArchivePresent,
			)
		}

		repoRoot := s.repoRootPath()
		report, err := prepareStartupRuntimeWithProgress(ctx, repoRoot, kind, func(event deps.PrepareProgress) {
			logStartupProgress(s.currentLogger(), repoRoot, event)
		})
		if err != nil {
			issue := startupFailureIssue(kind, err)
			s.setStartupRuntimeState(kind, StartupRuntimePhaseFailed, &issue)
			logStartupFailure(s.currentLogger(), repoRoot, kind, err)
			continue
		}

		s.setStartupRuntimeState(kind, StartupRuntimePhaseReady, nil)
		if kind == "chromium" && s.renderer != nil && report.PreparedEntrypoint != "" {
			s.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
		}
		if s.currentLogger() != nil {
			s.currentLogger().Info(
				"运行环境准备完成："+label,
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"used_cached_archive", report.UsedCachedArchive,
				"used_prepared_store", report.UsedPreparedStore,
				"used_system_browser", report.UsedSystemBrowser,
				"store_root", logpath.Display(repoRoot, report.StoreRoot),
			)
		}
	}

	s.ReconcileRecoverySummaryBestEffort("startup.runtime_prepare")
}

func (s *Service) AutoPrepareRuntimeEnvironments(ctx context.Context) {
	s.autoPrepareRuntimeEnvironments(ctx)
}

func logStartupFailure(logger *slog.Logger, repoRoot string, kind string, err error) {
	if logger == nil || err == nil {
		return
	}

	fields := []any{
		"component", "app",
		"resource_kind", kind,
	}

	var bootstrapErr *deps.BootstrapError
	pathValues := []string{repoRoot}
	if errors.As(err, &bootstrapErr) {
		pathValues = append(pathValues, bootstrapErr.ArchivePath, bootstrapErr.StoreRoot)
		fields = append(fields, "remediation", logpath.Text(repoRoot, bootstrapErr.Remediation, pathValues...))
	}

	label := runtimePrepareKindLabel(kind)
	logger.Warn(label+"运行环境准备失败，已跳过自动准备", append(fields, "err", logpath.Error(repoRoot, err, pathValues...))...)
}

func logStartupProgress(logger *slog.Logger, repoRoot string, event deps.PrepareProgress) {
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
		fields = append(fields, "archive_path", logpath.Display(repoRoot, event.ArchivePath))
	}
	if event.StoreRoot != "" {
		fields = append(fields, "store_root", logpath.Display(repoRoot, event.StoreRoot))
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
	message := runtimePrepareProgressMessage(event)
	if event.Status == "failed" {
		logger.Warn(message, fields...)
		return
	}
	logger.Info(message, fields...)
}

func runtimePrepareProgressMessage(event deps.PrepareProgress) string {
	label := strings.TrimSpace(event.Label)
	if label == "" {
		label = runtimePrepareKindLabel(event.Kind)
	}
	stage := runtimePrepareStageLabel(event.Stage)
	status := runtimePrepareStatusLabel(event.Status)
	if event.Summary != "" {
		return "运行环境准备：" + label + "，" + stage + status + "，" + event.Summary
	}
	return "运行环境准备：" + label + "，" + stage + status
}

func runtimePrepareKindLabel(kind string) string {
	switch strings.TrimSpace(kind) {
	case "chromium":
		return "图片渲染 Chromium"
	case "python", "python-runtime":
		return "Python 运行环境"
	case "node", "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		if kind = strings.TrimSpace(kind); kind != "" {
			return kind
		}
		return "运行环境"
	}
}

func runtimePrepareStageLabel(stage string) string {
	switch strings.TrimSpace(stage) {
	case "resolve":
		return "解析资源"
	case "download":
		return "下载资源"
	case "verify":
		return "校验资源"
	case "extract":
		return "解压资源"
	case "ready":
		return "准备完成"
	default:
		if stage = strings.TrimSpace(stage); stage != "" {
			return stage
		}
		return "处理"
	}
}

func runtimePrepareStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "started":
		return "已开始"
	case "running":
		return "进行中"
	case "succeeded":
		return "成功"
	case "failed":
		return "失败"
	case "skipped":
		return "已跳过"
	default:
		if status = strings.TrimSpace(status); status != "" {
			return status
		}
		return "进行中"
	}
}
