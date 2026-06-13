package recovery

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func EvaluateRestore(manifest BackupManifest, repoRoot string) CompatibilitySummary {
	now := time.Now().UTC().Format(time.RFC3339)
	summary := CompatibilitySummary{
		Status:                    "pending",
		Phase:                     "pre_restore",
		Operation:                 classifyOperation(manifest.CoreVersion, DetectCoreVersion(repoRoot)),
		CreatedAt:                 now,
		UpdatedAt:                 now,
		SourceCoreVersion:         manifest.CoreVersion,
		TargetCoreVersion:         DetectCoreVersion(repoRoot),
		SourceConfigSchemaVersion: manifest.ConfigSchemaVersion,
		TargetConfigSchemaVersion: config.CurrentSchemaVersion(),
		SourceDBSchemaVersion:     manifest.DBSchemaVersion,
		TargetDBSchemaVersion:     storage.CurrentSchemaVersion(),
		RequiresPostStartChecks:   true,
		NextSteps: []string{
			"重新启动服务以完成恢复后的兼容性检查。",
			"检查 recovery_summary 中列出的资源与插件处理建议。",
		},
	}

	if isSchemaNewer(manifest.ConfigSchemaVersion, config.CurrentSchemaVersion()) {
		summary.Status = "blocked"
		summary.RequiresPostStartChecks = false
		summary.Issues = append(summary.Issues, CompatibilityIssue{
			Code:        "recovery.config_schema_newer_than_target",
			Severity:    "error",
			Summary:     "备份的配置 schema 版本高于当前程序支持范围。",
			Remediation: "请使用与备份版本相同或更新的正式包执行恢复。",
		})
	}
	if isSchemaNewer(manifest.DBSchemaVersion, storage.CurrentSchemaVersion()) {
		summary.Status = "blocked"
		summary.RequiresPostStartChecks = false
		summary.Issues = append(summary.Issues, CompatibilityIssue{
			Code:        "recovery.db_schema_newer_than_target",
			Severity:    "error",
			Summary:     "备份的数据库 schema 版本高于当前程序支持范围。",
			Remediation: "请使用与备份版本相同或更新的正式包执行恢复。",
		})
	}

	if summary.Status != "blocked" {
		summary.Issues = append(summary.Issues, CompatibilityIssue{
			Code:        "recovery.post_start_checks_required",
			Severity:    "warning",
			Summary:     "恢复包已通过预检，仍需在下次启动时完成资源与插件兼容性检查。",
			Remediation: "启动服务后查看管理面、Launcher 或 diagnostics 中的恢复摘要。",
		})
	}

	return summary
}

func Finalize(summary CompatibilitySummary, input FinalizeInput) CompatibilitySummary {
	summary.Phase = "post_startup"
	summary.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	summary.RequiresPostStartChecks = false
	summary.Issues = nil
	summary.ManualActions = nil
	summary.NextSteps = nil
	summary.SkippedPlugins = nil
	summary.Audit = trimAuditEntries(summary.Audit)

	machineIssues := cloneIssues(input.Readiness.RuntimeIssues)
	summary.Issues = append(summary.Issues, machineIssues...)

	confirmedReviews := confirmedReviewLookup(summary)

	platformName := currentPlatform()
	for _, plugin := range input.Plugins {
		if plugin.SourceRoot == "plugins/builtin" || plugin.RegistrationState != "installed" {
			continue
		}
		reasonCode, skipped := pluginCompatibilityIssue(plugin, summary.TargetCoreVersion, platformName)
		if reasonCode == "" {
			continue
		}
		if confirmation, ok := confirmedReviews[skipped.ReviewID]; ok {
			skipped.ReviewStatus = reviewStatusConfirmed
			skipped.ReviewedAt = confirmation.ReviewedAt
			skipped.ReviewedBy = confirmation.ReviewedBy
		}
		summary.SkippedPlugins = append(summary.SkippedPlugins, skipped)
		if skipped.ReviewStatus != reviewStatusConfirmed {
			summary.Issues = append(summary.Issues, pluginIssueFromSkipped(skipped))
		}
		if input.DesiredStateRepo != nil && plugin.DesiredState != "disabled" {
			_ = input.DesiredStateRepo.SaveDesiredState(context.Background(), plugin.PluginID, "disabled", time.Now().UTC())
		}
	}

	pendingSkippedPlugins := pendingSkippedPlugins(summary.SkippedPlugins)
	summary.Status = recoveryStatus(machineIssues, pendingSkippedPlugins)
	if summary.Status != "compatible" {
		summary.ManualActions = buildManualActions(machineIssues, pendingSkippedPlugins)
		summary.NextSteps = buildNextSteps(machineIssues, pendingSkippedPlugins)
	}
	return summary
}
