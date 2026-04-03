package recovery

import (
	"context"
	"slices"
	"testing"
	"time"

	"rayleabot/server/internal/plugins"
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
						Remediation: "请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。",
					},
				},
			},
		},
	)

	if summary.Status != "degraded" {
		t.Fatalf("expected degraded summary, got %#v", summary)
	}
	if !slices.Equal(summary.ManualActions, []string{"请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。"}) {
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

func countString(items []string, target string) int {
	count := 0
	for _, item := range items {
		if item == target {
			count++
		}
	}
	return count
}
