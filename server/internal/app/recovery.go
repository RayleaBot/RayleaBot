package app

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type systemService struct {
	state            *appRuntimeState
	auth             *auth.Manager
	adapter          *adapter.Shell
	plugins          *plugins.Catalog
	runtimes         *runtimeRegistry
	renderer         *render.Service
	pluginRepository plugins.DesiredStateRepository
	taskExecutor     *tasks.Executor
	logRepository    logging.Repository
	shuttingDown     *atomic.Bool
	statusPublisher  *serviceStatusService
}

func newSystemService(deps systemServiceDeps) *systemService {
	return &systemService{
		state:            deps.state,
		auth:             deps.auth,
		adapter:          deps.adapter,
		plugins:          deps.plugins,
		runtimes:         deps.runtimes,
		renderer:         deps.renderer,
		pluginRepository: deps.pluginRepository,
		taskExecutor:     deps.taskExecutor,
		logRepository:    deps.logRepository,
	}
}

func (s *systemService) RefreshRecoverySummary() {
	if s == nil || s.state.repoRoot == "" {
		return
	}

	summary, err := recovery.LoadSummary(s.state.repoRoot)
	if err != nil || summary == nil {
		s.applyRecoverySummary(summary)
		return
	}
	if summary.RequiresPostStartChecks || recovery.NeedsSummaryNormalization(*summary) {
		reconciled, reconcileErr := s.reconcileRecoverySummary()
		if reconcileErr == nil && reconciled != nil {
			summary = reconciled
		}
	}
	s.applyRecoverySummary(summary)
}

func (s *systemService) renderDiagnostics() []recovery.CompatibilityIssue {
	if s == nil || s.renderer == nil {
		return nil
	}
	diagnostics := s.renderer.Diagnostics()
	if len(diagnostics) == 0 {
		return nil
	}
	items := make([]recovery.CompatibilityIssue, 0, len(diagnostics))
	for _, issue := range diagnostics {
		items = append(items, recovery.CompatibilityIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return items
}

func (s *systemService) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	if s == nil || s.state.repoRoot == "" {
		return nil
	}
	requiredKinds := startupManagedRuntimeDiagnosticKinds()
	if len(requiredKinds) == 0 {
		return nil
	}
	issues := []recovery.CompatibilityIssue{}
	manager := deps.NewManager(s.state.repoRoot)
	for _, kind := range requiredKinds {
		inspection, err := manager.Inspect(kind)
		if err != nil {
			issues = append(issues, runtimeInspectionIssue(kind, err))
			continue
		}
		if !inspection.MetadataComplete {
			issues = append(issues, runtimeMetadataIssue(kind))
			continue
		}
		if inspection.PreparedStorePresent {
			continue
		}
		if state, ok := s.startupRuntimeState(kind); ok {
			switch state.Phase {
			case startupRuntimePending:
				continue
			case startupRuntimeFailed:
				if state.Issue != nil {
					issues = append(issues, *state.Issue)
					continue
				}
			}
		}
		label := deps.ManagedResourceLabel(kind)
		summary := label + "尚未准备完成。"
		if inspection.CachedArchivePresent {
			summary = label + "归档已缓存，仍需展开运行时。"
		}
		issues = append(issues, recovery.CompatibilityIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     summary,
			Remediation: deps.BootstrapRemediation(kind, inspection.ArchivePath, inspection.StoreRoot),
		})
	}
	return issues
}

func runtimeInspectionIssue(_ string, err error) recovery.CompatibilityIssue {
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

func (s *systemService) platformDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	items := s.renderDiagnostics()
	items = append(items, s.managedRuntimeDiagnostics(pluginsList)...)
	if len(items) == 0 {
		return nil
	}
	return items
}

func (s *systemService) recoveryFinalizeInput() recovery.FinalizeInput {
	pluginsList := []plugins.Snapshot(nil)
	if s != nil && s.plugins != nil {
		pluginsList = s.plugins.List()
	}
	issues := s.platformDiagnostics(pluginsList)
	return recovery.FinalizeInput{
		Plugins:          pluginsList,
		DesiredStateRepo: s.pluginRepository,
		Readiness: recovery.RuntimeReadiness{
			RuntimeReady:  len(issues) == 0,
			RuntimeIssues: issues,
		},
	}
}

func (s *systemService) reconcileRecoverySummary() (*recovery.CompatibilitySummary, error) {
	if s == nil || s.state.repoRoot == "" {
		return nil, nil
	}
	summary, err := recovery.LoadSummary(s.state.repoRoot)
	if err != nil || summary == nil {
		return summary, err
	}
	if !summary.RequiresPostStartChecks && summary.Phase != "post_startup" {
		return nil, nil
	}

	reconciled := recovery.Finalize(*summary, s.recoveryFinalizeInput())
	if err := recovery.SaveSummary(s.state.repoRoot, reconciled); err != nil {
		return nil, err
	}
	s.applyRecoverySummary(&reconciled)
	return &reconciled, nil
}

func (s *systemService) ReconcileRecoverySummaryBestEffort(trigger string) {
	if s == nil {
		return
	}
	if _, err := s.reconcileRecoverySummary(); err != nil && s.state.Logger != nil {
		s.state.Logger.Warn(
			"failed to reconcile recovery summary",
			"component", "app",
			"trigger", strings.TrimSpace(trigger),
			"err", err.Error(),
		)
	}
}

func (s *systemService) applyRecoverySummary(summary *recovery.CompatibilitySummary) {
	if s == nil {
		return
	}
	if summary != nil && s.plugins != nil {
		for _, skipped := range summary.SkippedPlugins {
			if snapshot, ok := s.plugins.Get(skipped.PluginID); ok && snapshot.DesiredState != "disabled" {
				_, _ = s.plugins.SetDesiredState(skipped.PluginID, "disabled")
			}
		}
	}
	s.state.setRecoverySummary(summary)
	s.publishStatusSnapshot()
}

func (s *systemService) activePluginCount() int {
	if s == nil || s.runtimes == nil {
		return 0
	}
	return s.runtimes.ActiveCount()
}

func (s *systemService) uptimeSeconds() int64 {
	if s == nil || s.state == nil || s.state.startedAt.IsZero() {
		return 0
	}

	uptime := time.Since(s.state.startedAt)
	if uptime < 0 {
		return 0
	}

	return int64(uptime / time.Second)
}

func (s *systemService) systemStatus() string {
	if s != nil && s.shuttingDown != nil && s.shuttingDown.Load() {
		return "shutting_down"
	}
	return "running"
}

func (s *systemService) publishStatusSnapshot() {
	if s == nil || s.statusPublisher == nil {
		return
	}
	s.statusPublisher.PublishSnapshot()
}

func recoveryIssuesToHealth(issues []recovery.CompatibilityIssue) []health.DiagnosticIssue {
	if len(issues) == 0 {
		return nil
	}
	items := make([]health.DiagnosticIssue, 0, len(issues))
	for _, issue := range issues {
		items = append(items, health.DiagnosticIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return items
}

func runtimeMetadataIssue(kind string) recovery.CompatibilityIssue {
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

func containsRuntimeKind(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
