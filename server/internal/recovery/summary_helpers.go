package recovery

import "strings"

func buildManualActions(runtimeIssues []CompatibilityIssue, skippedPlugins []SkippedPlugin) []string {
	actions := make([]string, 0, len(runtimeIssues)+len(skippedPlugins)+1)
	for _, issue := range runtimeIssues {
		actions = appendUniqueString(actions, strings.TrimSpace(issue.Remediation))
	}
	for _, skipped := range skippedPlugins {
		actions = appendUniqueString(actions, strings.TrimSpace(skipped.ManualAction))
	}
	if len(skippedPlugins) > 0 {
		actions = appendUniqueString(actions, "处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。")
	}
	if len(actions) == 0 {
		return nil
	}
	return actions
}

func buildNextSteps(runtimeIssues []CompatibilityIssue, skippedPlugins []SkippedPlugin) []string {
	steps := make([]string, 0, 4)
	if len(runtimeIssues) > 0 {
		steps = appendUniqueString(steps, "完成上述兼容性处理后，重启服务并确认恢复摘要变为 compatible。")
	}
	if len(skippedPlugins) > 0 {
		steps = appendUniqueString(steps, "查看恢复摘要中的跳过插件列表并完成兼容性处理。")
		steps = appendUniqueString(steps, "处理完成后，在管理面中手动重新启用被跳过插件。")
	}
	steps = appendUniqueString(steps, "通过管理面、Launcher 或 diagnostics 复核 recovery_summary。")
	if len(steps) == 0 {
		return nil
	}
	return steps
}

func NeedsSummaryNormalization(summary CompatibilitySummary) bool {
	if summary.Phase != "post_startup" {
		return false
	}
	if len(summary.Audit) > maxAuditEntries {
		return true
	}
	for _, skipped := range summary.SkippedPlugins {
		if strings.TrimSpace(skipped.ReviewID) == "" {
			return true
		}
		switch skipped.ReviewStatus {
		case reviewStatusPending, reviewStatusConfirmed:
		default:
			return true
		}
	}
	return false
}
