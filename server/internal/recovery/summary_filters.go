package recovery

import "strings"

func issuesForSkippedPlugins(skippedPlugins []SkippedPlugin) []CompatibilityIssue {
	if len(skippedPlugins) == 0 {
		return nil
	}
	issues := make([]CompatibilityIssue, 0, len(skippedPlugins))
	for _, skipped := range skippedPlugins {
		issues = append(issues, pluginIssueFromSkipped(skipped))
	}
	return issues
}

func pendingSkippedPlugins(skippedPlugins []SkippedPlugin) []SkippedPlugin {
	if len(skippedPlugins) == 0 {
		return nil
	}
	items := make([]SkippedPlugin, 0, len(skippedPlugins))
	for _, skipped := range skippedPlugins {
		if skipped.ReviewStatus == reviewStatusConfirmed {
			continue
		}
		items = append(items, skipped)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func filterMachineIssues(issues []CompatibilityIssue) []CompatibilityIssue {
	if len(issues) == 0 {
		return nil
	}
	filtered := make([]CompatibilityIssue, 0, len(issues))
	for _, issue := range issues {
		if isPluginRecoveryIssueCode(issue.Code) {
			continue
		}
		filtered = append(filtered, issue)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func isPluginRecoveryIssueCode(code string) bool {
	return strings.HasPrefix(strings.TrimSpace(code), "recovery.plugin_")
}

func recoveryStatus(machineIssues []CompatibilityIssue, pendingSkipped []SkippedPlugin) string {
	for _, issue := range machineIssues {
		if issue.Severity == "error" {
			return "blocked"
		}
	}
	if len(machineIssues) > 0 || len(pendingSkipped) > 0 {
		return "degraded"
	}
	return "compatible"
}
