package recovery

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func SummaryPath(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(RecoverySummaryPath))
}

func LoadSummary(repoRoot string) (*CompatibilitySummary, error) {
	path := SummaryPath(repoRoot)
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var summary CompatibilitySummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func SaveSummary(repoRoot string, summary CompatibilitySummary) error {
	path := SummaryPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func RemoveSummary(repoRoot string) error {
	err := os.Remove(SummaryPath(repoRoot))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func HasSummary(repoRoot string) bool {
	_, err := os.Stat(SummaryPath(repoRoot))
	return err == nil
}

func LoadSummaryFromFile(path string) (*CompatibilitySummary, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var summary CompatibilitySummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func SaveSummaryToFile(path string, summary CompatibilitySummary) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func AvailableRecoveryLogFiles(logDir string) []fs.DirEntry {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}
	return entries
}

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

func trimAuditEntries(entries []AuditEntry) []AuditEntry {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) > maxAuditEntries {
		entries = entries[:maxAuditEntries]
	}
	cloned := make([]AuditEntry, 0, len(entries))
	for _, entry := range entries {
		items := make([]AuditItem, 0, len(entry.Items))
		for _, item := range entry.Items {
			items = append(items, item)
		}
		cloned = append(cloned, AuditEntry{
			TaskID:     entry.TaskID,
			CreatedAt:  entry.CreatedAt,
			OperatorID: entry.OperatorID,
			Note:       entry.Note,
			Items:      items,
		})
	}
	return cloned
}

func cloneIssues(issues []CompatibilityIssue) []CompatibilityIssue {
	if len(issues) == 0 {
		return nil
	}
	cloned := make([]CompatibilityIssue, 0, len(issues))
	for _, issue := range issues {
		cloned = append(cloned, CompatibilityIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return cloned
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || contains(items, value) {
		return items
	}
	return append(items, value)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		text := stringValue(item)
		if text != "" {
			items = append(items, text)
		}
	}
	return items
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
