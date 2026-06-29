package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/system/model"
	"github.com/RayleaBot/RayleaBot/server/internal/system/startup"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *Service) DiagnosticsSnapshot(ctx context.Context) model.DiagnosticsSnapshot {
	now := time.Now().UTC()
	status := s.StatusSnapshot()
	readiness := s.CurrentReadiness()
	summary := s.summary()

	database, databaseIssues := s.diagnosticsDatabase(ctx)
	render := s.diagnosticsRender()
	thirdParty, thirdPartyIssues := s.diagnosticsThirdParty(ctx)
	dependencies, dependencyIssues := s.diagnosticsDependencies()
	filesystem := s.diagnosticsFilesystem(summary)
	recentErrors, logIssues := s.diagnosticsRecentErrors(ctx)

	issues := append([]health.DiagnosticIssue{}, readiness.Issues...)
	issues = append(issues, render.Issues...)
	issues = append(issues, databaseIssues...)
	issues = append(issues, thirdPartyIssues...)
	issues = append(issues, dependencyIssues...)
	issues = append(issues, logIssues...)

	return model.DiagnosticsSnapshot{
		GeneratedAt: now.Format(time.RFC3339),
		Build: model.DiagnosticsBuild{
			CoreVersion: recovery.DetectCoreVersion(s.repoRootPath()),
		},
		System: model.DiagnosticsSystem{
			Status:        status.Status,
			UptimeSeconds: status.UptimeSeconds,
		},
		Config: model.DiagnosticsConfig{
			SchemaVersion:    config.CurrentSchemaVersion(),
			Status:           "loaded",
			ApplyState:       "applied",
			ConfigPath:       summary.ConfigPath,
			SchemaPath:       summary.SchemaPath,
			DatabaseEngine:   summary.DatabaseEngine,
			DatabasePath:     summary.DatabasePath,
			OneBotConfigured: summary.OneBotConfigured,
		},
		Secrets: model.DiagnosticsSecrets{
			UnresolvedRefs: []string{},
		},
		Database: database,
		Adapter: model.DiagnosticsAdapter{
			State: status.AdapterState,
		},
		Plugins: model.DiagnosticsPlugins{
			Total:   s.pluginCount(),
			Active:  status.ActivePlugins,
			Running: status.RunningPlugins,
			Failed:  status.FailedPlugins,
		},
		Render:          render,
		ThirdParty:      thirdParty,
		Scheduler:       s.diagnosticsScheduler(),
		Tasks:           s.diagnosticsTasks(),
		Dependencies:    dependencies,
		Filesystem:      filesystem,
		RecentErrors:    recentErrors,
		Issues:          dedupeDiagnosticIssues(issues),
		RecoverySummary: status.RecoverySummary,
	}
}

func (s *Service) pluginCount() int {
	if s == nil || s.plugins == nil {
		return 0
	}
	return len(s.plugins.List())
}

func (s *Service) diagnosticsDatabase(ctx context.Context) (model.DiagnosticsDatabase, []health.DiagnosticIssue) {
	result := model.DiagnosticsDatabase{
		SchemaVersion:     s.dbSchemaVersion(),
		AppliedMigrations: []model.DiagnosticsMigration{},
	}
	if s == nil || s.storage == nil || s.storage.Read == nil {
		return result, []health.DiagnosticIssue{{
			Code:        "storage.schema_migrations_unavailable",
			Severity:    "warning",
			Summary:     "数据库迁移记录不可用",
			Remediation: "请确认数据库已打开，并检查服务启动日志中的 SQLite 初始化错误。",
		}}
	}

	migrations, err := s.storage.ListAppliedMigrations(ctx)
	if err != nil {
		return result, []health.DiagnosticIssue{{
			Code:        "storage.schema_migrations_unavailable",
			Severity:    "warning",
			Summary:     "数据库迁移记录不可读",
			Remediation: "请检查 SQLite 文件权限和 schema_migrations 表是否完整。",
		}}
	}

	for _, migration := range migrations {
		result.AppliedMigrations = append(result.AppliedMigrations, model.DiagnosticsMigration{
			Version:   fmt.Sprintf("%06d", migration.Version),
			Name:      migration.Name,
			AppliedAt: migration.AppliedAt,
		})
	}
	return result, nil
}

func (s *Service) diagnosticsRender() model.DiagnosticsIssueGroup {
	issues := recoveryIssuesToHealth(s.renderDiagnostics())
	status := "ok"
	if len(issues) > 0 {
		status = "degraded"
	}
	return model.DiagnosticsIssueGroup{
		Status: status,
		Issues: nonNilIssues(issues),
	}
}

func (s *Service) diagnosticsThirdParty(ctx context.Context) (model.DiagnosticsThirdParty, []health.DiagnosticIssue) {
	if s == nil || s.thirdParty == nil {
		return model.DiagnosticsThirdParty{Platforms: []model.DiagnosticsThirdPartyPlatform{}}, nil
	}
	return s.thirdParty.DiagnosticsThirdParty(ctx)
}

func (s *Service) diagnosticsScheduler() model.DiagnosticsScheduler {
	if s == nil || s.scheduler == nil {
		return model.DiagnosticsScheduler{}
	}
	return s.scheduler.DiagnosticsScheduler()
}

func (s *Service) diagnosticsTasks() model.DiagnosticsTaskSummary {
	result := model.DiagnosticsTaskSummary{}
	if s == nil || s.taskExecutor == nil {
		return result
	}
	for _, task := range s.taskExecutor.List() {
		switch task.Status {
		case tasks.StatusPending:
			result.Pending++
		case tasks.StatusRunning:
			result.Running++
		case tasks.StatusFailed, tasks.StatusInterrupted:
			result.Failed++
		}
	}
	return result
}

func (s *Service) diagnosticsDependencies() ([]model.DiagnosticsDependency, []health.DiagnosticIssue) {
	if s == nil || strings.TrimSpace(s.repoRootPath()) == "" {
		return []model.DiagnosticsDependency{}, nil
	}
	diagnostics := deps.NewDiagnostics(s.repoRootPath())
	kinds := startup.Kinds()
	items := make([]model.DiagnosticsDependency, 0, len(kinds))
	issues := []health.DiagnosticIssue{}
	for _, kind := range kinds {
		item := model.DiagnosticsDependency{Kind: kind, Status: "unavailable"}
		inspection, err := diagnostics.InspectRuntime(kind)
		if err != nil {
			var bootstrapErr *deps.BootstrapError
			remediation := "请检查 .deps/manifest.json 和本机依赖缓存。"
			summary := deps.ManagedResourceLabel(kind) + "不可用"
			if errors.As(err, &bootstrapErr) {
				remediation = bootstrapErr.Remediation
				summary = bootstrapErr.Message
			}
			issues = append(issues, health.DiagnosticIssue{
				Code:        "dependency." + kind,
				Severity:    "warning",
				Summary:     summary,
				Remediation: remediation,
			})
			items = append(items, item)
			continue
		}
		item.MetadataComplete = inspection.MetadataComplete
		item.CachedArchivePresent = inspection.CachedArchivePresent
		item.PreparedStorePresent = inspection.PreparedStorePresent
		item.SystemBrowser = strings.TrimSpace(inspection.SystemBrowserPath) != ""
		item.Status = dependencyStatus(inspection)
		items = append(items, item)
	}
	return items, issues
}

func dependencyStatus(inspection *deps.BootstrapInspection) string {
	if inspection == nil {
		return "unavailable"
	}
	if !inspection.MetadataComplete {
		return "metadata_incomplete"
	}
	if inspection.PreparedStorePresent || strings.TrimSpace(inspection.SystemBrowserPath) != "" {
		return "ready"
	}
	if inspection.CachedArchivePresent {
		return "cached"
	}
	return "on_demand"
}

func (s *Service) diagnosticsFilesystem(summary config.Summary) []model.DiagnosticsPathPermission {
	paths := []model.DiagnosticsPathPermission{
		pathPermission("repo_root", s.repoRootPath()),
		pathPermission("config", summary.ConfigPath),
	}
	if databasePath, err := s.databasePath(summary.ConfigPath, s.config().Database.Path); err == nil {
		paths = append(paths, pathPermission("database", databasePath))
		paths = append(paths, pathPermission("logs", filepath.Dir(logging.SpoolPathForDatabase(databasePath))))
	}
	paths = append(paths, pathPermission("plugins", filepath.Join(s.repoRootPath(), "plugins")))
	return paths
}

func pathPermission(label, path string) model.DiagnosticsPathPermission {
	item := model.DiagnosticsPathPermission{
		Label:  label,
		Path:   path,
		Status: "unknown",
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return item
	}
	info, err := os.Stat(path)
	if err == nil {
		item.Status = "ok"
		item.IsDir = info.IsDir()
		return item
	}
	if os.IsNotExist(err) {
		item.Status = "missing"
		return item
	}
	item.Status = "unreadable"
	return item
}

func (s *Service) diagnosticsRecentErrors(ctx context.Context) ([]logging.Summary, []health.DiagnosticIssue) {
	if s == nil || s.logRepository == nil {
		return []logging.Summary{}, nil
	}
	items, err := s.logRepository.ListSummaries(ctx, logging.Query{Levels: []string{"error"}, Limit: 20})
	if err != nil {
		return []logging.Summary{}, []health.DiagnosticIssue{{
			Code:        "logging.recent_errors_unavailable",
			Severity:    "warning",
			Summary:     "近期错误日志不可读",
			Remediation: "请检查管理日志数据库表和日志保留配置。",
		}}
	}
	if items == nil {
		items = []logging.Summary{}
	}
	return items, nil
}

func nonNilIssues(items []health.DiagnosticIssue) []health.DiagnosticIssue {
	if items == nil {
		return []health.DiagnosticIssue{}
	}
	result := make([]health.DiagnosticIssue, 0, len(items))
	for _, item := range items {
		result = append(result, normalizeDiagnosticIssue(item))
	}
	return result
}

func normalizeDiagnosticIssue(item health.DiagnosticIssue) health.DiagnosticIssue {
	if strings.TrimSpace(item.UserMessage) == "" {
		item.UserMessage = item.Summary
	}
	if strings.TrimSpace(item.InternalReason) == "" {
		item.InternalReason = item.Code
	}
	return item
}

func dedupeDiagnosticIssues(items []health.DiagnosticIssue) []health.DiagnosticIssue {
	if len(items) == 0 {
		return []health.DiagnosticIssue{}
	}
	seen := map[string]struct{}{}
	result := make([]health.DiagnosticIssue, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Code) == "" || strings.TrimSpace(item.Summary) == "" {
			continue
		}
		key := item.Code + "\x00" + item.Summary
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(item.Severity) == "" {
			item.Severity = "warning"
		}
		result = append(result, normalizeDiagnosticIssue(item))
	}
	if result == nil {
		return []health.DiagnosticIssue{}
	}
	return result
}
