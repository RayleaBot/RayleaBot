package recovery

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type desiredStateRepoStub struct {
	saves []string
}

func (s *desiredStateRepoStub) LoadDesiredStates(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *desiredStateRepoStub) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	s.saves = append(s.saves, pluginID+":"+desiredState)
	return nil
}

func (s *desiredStateRepoStub) DeleteDesiredState(context.Context, string) error {
	return nil
}

func TestFinalizeClearsPostStartCheckIssueForCompatibleSummary(t *testing.T) {
	t.Parallel()

	summary := Finalize(
		CompatibilitySummary{
			Status:                  "pending",
			Phase:                   "pre_restore",
			TargetCoreVersion:       "0.2.0",
			RequiresPostStartChecks: true,
			Issues: []CompatibilityIssue{
				{
					Code:        "recovery.post_start_checks_required",
					Severity:    "warning",
					Summary:     "恢复包已通过预检，仍需在下次启动时完成资源与插件兼容性检查。",
					Remediation: "启动服务后查看管理面、Launcher 或 diagnostics 中的恢复摘要。",
				},
			},
			ManualActions: []string{"stale action"},
			NextSteps:     []string{"stale step"},
			SkippedPlugins: []SkippedPlugin{
				{PluginID: "stale-plugin"},
			},
		},
		FinalizeInput{
			Readiness: RuntimeReadiness{RuntimeReady: true},
		},
	)

	if summary.Status != "compatible" {
		t.Fatalf("expected compatible summary, got %#v", summary)
	}
	if slices.ContainsFunc(summary.Issues, func(issue CompatibilityIssue) bool {
		return issue.Code == "recovery.post_start_checks_required"
	}) {
		t.Fatalf("compatible summary should not retain pre-restore issue: %#v", summary.Issues)
	}
	if len(summary.ManualActions) != 0 {
		t.Fatalf("compatible summary should not retain manual actions: %#v", summary.ManualActions)
	}
	if len(summary.NextSteps) != 0 {
		t.Fatalf("compatible summary should not retain next steps: %#v", summary.NextSteps)
	}
	if len(summary.SkippedPlugins) != 0 {
		t.Fatalf("compatible summary should not retain skipped plugins: %#v", summary.SkippedPlugins)
	}
}

func TestFinalizeBuildsRuntimeGuidance(t *testing.T) {
	t.Parallel()

	summary := Finalize(
		CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
			Issues: []CompatibilityIssue{
				{
					Code:        "recovery.post_start_checks_required",
					Severity:    "warning",
					Summary:     "恢复包已通过预检，仍需在下次启动时完成资源与插件兼容性检查。",
					Remediation: "启动服务后查看管理面、Launcher 或 diagnostics 中的恢复摘要。",
				},
			},
		},
		FinalizeInput{
			Readiness: RuntimeReadiness{
				RuntimeReady: false,
				RuntimeIssues: []CompatibilityIssue{
					{
						Code:        "platform.resource_missing",
						Severity:    "warning",
						Summary:     "Chromium 资源尚未准备完成。",
						Remediation: "请先准备 Chromium 浏览环境，或在配置中显式设置 render.browser_path。",
					},
				},
			},
		},
	)

	if summary.Status != "degraded" {
		t.Fatalf("expected degraded summary, got %#v", summary)
	}
	if !slices.Equal(summary.ManualActions, []string{"请先准备 Chromium 浏览环境，或在配置中显式设置 render.browser_path。"}) {
		t.Fatalf("unexpected runtime manual actions: %#v", summary.ManualActions)
	}
	expectedSteps := []string{
		"完成上述兼容性处理后，重启服务并确认恢复摘要变为 compatible。",
		"通过管理面、Launcher 或 diagnostics 复核 recovery_summary。",
	}
	if !slices.Equal(summary.NextSteps, expectedSteps) {
		t.Fatalf("unexpected runtime next steps: %#v", summary.NextSteps)
	}
}

func TestFinalizeBuildsPluginGuidanceAndDisablesSkippedPlugins(t *testing.T) {
	t.Parallel()

	repo := &desiredStateRepoStub{}
	summary := Finalize(
		CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
			Issues: []CompatibilityIssue{
				{
					Code:        "recovery.post_start_checks_required",
					Severity:    "warning",
					Summary:     "恢复包已通过预检，仍需在下次启动时完成资源与插件兼容性检查。",
					Remediation: "启动服务后查看管理面、Launcher 或 diagnostics 中的恢复摘要。",
				},
			},
		},
		FinalizeInput{
			DesiredStateRepo: repo,
			Readiness:        RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.4.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "enabled",
				},
				{
					PluginID:          "arm-only",
					Version:           "1.0.0",
					Platforms:         []string{"linux-arm64"},
					ManifestPath:      "plugins/installed/arm-only/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "enabled",
				},
			},
		},
	)

	if summary.Status != "degraded" {
		t.Fatalf("expected degraded summary, got %#v", summary)
	}
	if got, want := len(summary.SkippedPlugins), 2; got != want {
		t.Fatalf("unexpected skipped plugin count: got %d want %d", got, want)
	}
	for _, skipped := range summary.SkippedPlugins {
		if skipped.ReviewID == "" || skipped.ReviewStatus != "pending" {
			t.Fatalf("expected pending review metadata on skipped plugin, got %#v", skipped)
		}
	}
	expectedActions := []string{
		"升级程序或重新安装兼容版本插件。",
		"安装支持当前平台的插件包。",
		"处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。",
	}
	if !slices.Equal(summary.ManualActions, expectedActions) {
		t.Fatalf("unexpected plugin manual actions: %#v", summary.ManualActions)
	}
	expectedSteps := []string{
		"查看恢复摘要中的跳过插件列表并完成兼容性处理。",
		"处理完成后，在管理面中手动重新启用被跳过插件。",
		"通过管理面、Launcher 或 diagnostics 复核 recovery_summary。",
	}
	if !slices.Equal(summary.NextSteps, expectedSteps) {
		t.Fatalf("unexpected plugin next steps: %#v", summary.NextSteps)
	}
	if !slices.Equal(repo.saves, []string{"weather-pro:disabled", "arm-only:disabled"}) {
		t.Fatalf("unexpected desired state writes: %#v", repo.saves)
	}
}

func TestFinalizeMergesRuntimeAndPluginGuidanceWithoutDuplicates(t *testing.T) {
	t.Parallel()

	summary := Finalize(
		CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
			Issues: []CompatibilityIssue{
				{
					Code:        "recovery.post_start_checks_required",
					Severity:    "warning",
					Summary:     "恢复包已通过预检，仍需在下次启动时完成资源与插件兼容性检查。",
					Remediation: "启动服务后查看管理面、Launcher 或 diagnostics 中的恢复摘要。",
				},
			},
		},
		FinalizeInput{
			Readiness: RuntimeReadiness{
				RuntimeReady: false,
				RuntimeIssues: []CompatibilityIssue{
					{
						Code:        "platform.resource_missing",
						Severity:    "warning",
						Summary:     "Chromium 资源尚未准备完成。",
						Remediation: "通过管理面、Launcher 或 diagnostics 复核 recovery_summary。",
					},
				},
			},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.4.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "enabled",
				},
			},
		},
	)

	if occurrences := countString(summary.NextSteps, "通过管理面、Launcher 或 diagnostics 复核 recovery_summary。"); occurrences != 1 {
		t.Fatalf("expected deduplicated review step once, got %d in %#v", occurrences, summary.NextSteps)
	}
}

func TestFinalizePreservesConfirmedReviewStateAcrossRecheck(t *testing.T) {
	t.Parallel()

	initial := Finalize(
		CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
		},
		FinalizeInput{
			Readiness: RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.4.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "disabled",
				},
			},
		},
	)

	confirmed, confirmedIDs, err := ConfirmSkippedPlugins(initial, []string{initial.SkippedPlugins[0].ReviewID}, "alice", "已确认当前跳过状态。", "task_confirm_0001")
	if err != nil {
		t.Fatalf("confirm skipped plugin: %v", err)
	}
	if !slices.Equal(confirmedIDs, []string{initial.SkippedPlugins[0].ReviewID}) {
		t.Fatalf("unexpected confirmed review ids: %#v", confirmedIDs)
	}
	if confirmed.Status != "compatible" {
		t.Fatalf("expected compatible summary after confirmation, got %#v", confirmed)
	}
	if len(confirmed.Audit) != 1 {
		t.Fatalf("expected one audit entry, got %#v", confirmed.Audit)
	}

	reconciled := Finalize(
		confirmed,
		FinalizeInput{
			Readiness: RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.4.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "disabled",
				},
			},
		},
	)

	if reconciled.Status != "compatible" {
		t.Fatalf("expected compatible summary after recheck, got %#v", reconciled)
	}
	if len(reconciled.SkippedPlugins) != 1 {
		t.Fatalf("expected skipped plugin to remain listed, got %#v", reconciled.SkippedPlugins)
	}
	if reconciled.SkippedPlugins[0].ReviewStatus != "confirmed" || reconciled.SkippedPlugins[0].ReviewedBy != "alice" {
		t.Fatalf("expected confirmed review to survive recheck, got %#v", reconciled.SkippedPlugins[0])
	}
	if len(reconciled.Issues) != 0 || len(reconciled.ManualActions) != 0 || len(reconciled.NextSteps) != 0 {
		t.Fatalf("expected compatible summary to clear unresolved guidance, got %#v", reconciled)
	}
}

func TestFinalizeResetsReviewWhenPluginVersionChanges(t *testing.T) {
	t.Parallel()

	initial := Finalize(
		CompatibilitySummary{
			Status:            "pending",
			Phase:             "pre_restore",
			TargetCoreVersion: "0.2.0",
		},
		FinalizeInput{
			Readiness: RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.4.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "disabled",
				},
			},
		},
	)
	confirmed, _, err := ConfirmSkippedPlugins(initial, []string{initial.SkippedPlugins[0].ReviewID}, "alice", "", "task_confirm_0001")
	if err != nil {
		t.Fatalf("confirm skipped plugin: %v", err)
	}

	reconciled := Finalize(
		confirmed,
		FinalizeInput{
			Readiness: RuntimeReadiness{RuntimeReady: true},
			Plugins: []plugins.Snapshot{
				{
					PluginID:          "weather-pro",
					Version:           "1.5.0",
					MinCoreVersion:    "0.3.0",
					ManifestPath:      "plugins/installed/weather-pro/info.json",
					SourceRoot:        "plugins/installed",
					RegistrationState: "installed",
					DesiredState:      "disabled",
				},
			},
		},
	)

	if reconciled.Status != "degraded" {
		t.Fatalf("expected degraded summary when review key changes, got %#v", reconciled)
	}
	if len(reconciled.SkippedPlugins) != 1 {
		t.Fatalf("expected one skipped plugin, got %#v", reconciled.SkippedPlugins)
	}
	if reconciled.SkippedPlugins[0].ReviewStatus != "pending" || reconciled.SkippedPlugins[0].ReviewedBy != "" {
		t.Fatalf("expected review state reset for changed version, got %#v", reconciled.SkippedPlugins[0])
	}
}

func TestConfirmSkippedPluginsIsIdempotent(t *testing.T) {
	t.Parallel()

	summary := CompatibilitySummary{
		Status: "degraded",
		Phase:  "post_startup",
		SkippedPlugins: []SkippedPlugin{
			{
				PluginID:     "weather-pro",
				Version:      "1.4.0",
				ReasonCode:   "plugin.min_core_version",
				Summary:      "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
				ReviewID:     buildReviewID("weather-pro", "plugin.min_core_version", "1.4.0"),
				ReviewStatus: "confirmed",
				ReviewedAt:   "2026-04-04T08:00:00Z",
				ReviewedBy:   "alice",
				ManualAction: "升级程序或重新安装兼容版本插件。",
			},
		},
		Audit: []AuditEntry{{
			TaskID:     "task_confirm_0001",
			CreatedAt:  "2026-04-04T08:00:00Z",
			OperatorID: "alice",
			Note:       "",
			Items: []AuditItem{{
				ReviewID:   buildReviewID("weather-pro", "plugin.min_core_version", "1.4.0"),
				PluginID:   "weather-pro",
				ReasonCode: "plugin.min_core_version",
				Summary:    "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
				Version:    "1.4.0",
			}},
		}},
	}

	updated, confirmedIDs, err := ConfirmSkippedPlugins(summary, []string{summary.SkippedPlugins[0].ReviewID}, "alice", "", "task_confirm_0002")
	if err != nil {
		t.Fatalf("confirm skipped plugin: %v", err)
	}
	if len(confirmedIDs) != 0 {
		t.Fatalf("expected idempotent confirmation to skip duplicates, got %#v", confirmedIDs)
	}
	if len(updated.Audit) != 1 {
		t.Fatalf("expected audit log to stay deduplicated, got %#v", updated.Audit)
	}
}

func TestConfirmSkippedPluginsRejectsUnknownReviewID(t *testing.T) {
	t.Parallel()

	_, _, err := ConfirmSkippedPlugins(
		CompatibilitySummary{
			Status: "degraded",
			Phase:  "post_startup",
			SkippedPlugins: []SkippedPlugin{{
				PluginID:     "weather-pro",
				ReasonCode:   "plugin.min_core_version",
				Summary:      "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
				ReviewID:     buildReviewID("weather-pro", "plugin.min_core_version", "1.4.0"),
				ReviewStatus: "pending",
			}},
		},
		[]string{"review_missing"},
		"alice",
		"",
		"task_confirm_0001",
	)
	var unknownErr *UnknownReviewIDsError
	if err == nil || !slices.Equal(unknownErrIDs(err, &unknownErr), []string{"review_missing"}) {
		t.Fatalf("expected unknown review id error, got %v", err)
	}
}

func unknownErrIDs(err error, target **UnknownReviewIDsError) []string {
	if err == nil {
		return nil
	}
	if errors.As(err, target) {
		return (*target).ReviewIDs
	}
	return nil
}

func countString(items []string, target string) int {
	count := 0
	for _, item := range items {
		if item == target {
			count++
		}
	}
	return count
}
