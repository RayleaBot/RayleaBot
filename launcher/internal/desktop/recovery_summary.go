package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type recoveryCompatibilityIssue struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type recoveryCompatibilitySkippedPlugin struct {
	PluginID     string `json:"plugin_id"`
	Version      string `json:"version,omitempty"`
	ReasonCode   string `json:"reason_code"`
	Summary      string `json:"summary"`
	ReviewID     string `json:"review_id"`
	ReviewStatus string `json:"review_status"`
	ReviewedAt   string `json:"reviewed_at,omitempty"`
	ReviewedBy   string `json:"reviewed_by,omitempty"`
	ManualAction string `json:"manual_action,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
}

type recoveryCompatibilityAuditItem struct {
	ReviewID   string `json:"review_id"`
	PluginID   string `json:"plugin_id"`
	ReasonCode string `json:"reason_code"`
	Summary    string `json:"summary"`
	Version    string `json:"version,omitempty"`
}

type recoveryCompatibilityAuditEntry struct {
	TaskID     string                            `json:"task_id"`
	CreatedAt  string                            `json:"created_at"`
	OperatorID string                            `json:"operator_id"`
	Note       *string                           `json:"note"`
	Items      *[]recoveryCompatibilityAuditItem `json:"items"`
}

type recoveryCompatibilitySummary struct {
	Status                    string                               `json:"status"`
	Phase                     string                               `json:"phase"`
	Operation                 string                               `json:"operation"`
	CreatedAt                 string                               `json:"created_at"`
	UpdatedAt                 string                               `json:"updated_at"`
	SourceCoreVersion         string                               `json:"source_core_version,omitempty"`
	TargetCoreVersion         string                               `json:"target_core_version,omitempty"`
	SourceConfigSchemaVersion string                               `json:"source_config_schema_version,omitempty"`
	TargetConfigSchemaVersion string                               `json:"target_config_schema_version,omitempty"`
	SourceDBSchemaVersion     string                               `json:"source_db_schema_version,omitempty"`
	TargetDBSchemaVersion     string                               `json:"target_db_schema_version,omitempty"`
	RequiresPostStartChecks   *bool                                `json:"requires_post_start_checks,omitempty"`
	Issues                    []recoveryCompatibilityIssue         `json:"issues,omitempty"`
	SkippedPlugins            []recoveryCompatibilitySkippedPlugin `json:"skipped_plugins,omitempty"`
	ManualActions             []string                             `json:"manual_actions,omitempty"`
	NextSteps                 []string                             `json:"next_steps,omitempty"`
	Audit                     []recoveryCompatibilityAuditEntry    `json:"audit,omitempty"`
}

func readRecoverySummary(logDirectory string) any {
	payload, err := os.ReadFile(filepath.Join(logDirectory, "recovery-summary.json"))
	if err != nil {
		return nil
	}
	return parseRecoverySummary(payload)
}

func parseRecoverySummary(payload []byte) JSONObject {
	var raw any
	if json.Unmarshal(payload, &raw) != nil || containsJSONNull(raw) {
		return nil
	}

	var summary recoveryCompatibilitySummary
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&summary) != nil {
		return nil
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil
	}
	if !summary.normalizeAndValidate() {
		return nil
	}

	normalized, err := json.Marshal(summary)
	if err != nil {
		return nil
	}
	var result JSONObject
	if json.Unmarshal(normalized, &result) != nil {
		return nil
	}
	return result
}

func (summary *recoveryCompatibilitySummary) normalizeAndValidate() bool {
	summary.Status = strings.TrimSpace(summary.Status)
	summary.Phase = strings.TrimSpace(summary.Phase)
	summary.Operation = strings.TrimSpace(summary.Operation)
	summary.CreatedAt = strings.TrimSpace(summary.CreatedAt)
	summary.UpdatedAt = strings.TrimSpace(summary.UpdatedAt)
	if !oneOf(summary.Status, "pending", "compatible", "degraded", "blocked") ||
		!oneOf(summary.Phase, "pre_restore", "post_startup") ||
		!oneOf(summary.Operation, "restore", "upgrade", "rollback") ||
		!validRecoveryDateTime(summary.CreatedAt) ||
		!validRecoveryDateTime(summary.UpdatedAt) {
		return false
	}

	summary.SourceCoreVersion = strings.TrimSpace(summary.SourceCoreVersion)
	summary.TargetCoreVersion = strings.TrimSpace(summary.TargetCoreVersion)
	summary.SourceConfigSchemaVersion = strings.TrimSpace(summary.SourceConfigSchemaVersion)
	summary.TargetConfigSchemaVersion = strings.TrimSpace(summary.TargetConfigSchemaVersion)
	summary.SourceDBSchemaVersion = strings.TrimSpace(summary.SourceDBSchemaVersion)
	summary.TargetDBSchemaVersion = strings.TrimSpace(summary.TargetDBSchemaVersion)
	summary.ManualActions = normalizedNonEmptyStrings(summary.ManualActions)
	summary.NextSteps = normalizedNonEmptyStrings(summary.NextSteps)

	for index := range summary.Issues {
		issue := &summary.Issues[index]
		issue.Code = strings.TrimSpace(issue.Code)
		issue.Severity = strings.TrimSpace(issue.Severity)
		issue.Summary = strings.TrimSpace(issue.Summary)
		issue.Remediation = strings.TrimSpace(issue.Remediation)
		if issue.Code == "" || issue.Summary == "" || !oneOf(issue.Severity, "warning", "error") {
			return false
		}
	}
	for index := range summary.SkippedPlugins {
		plugin := &summary.SkippedPlugins[index]
		plugin.PluginID = strings.TrimSpace(plugin.PluginID)
		plugin.Version = strings.TrimSpace(plugin.Version)
		plugin.ReasonCode = strings.TrimSpace(plugin.ReasonCode)
		plugin.Summary = strings.TrimSpace(plugin.Summary)
		plugin.ReviewID = strings.TrimSpace(plugin.ReviewID)
		plugin.ReviewStatus = strings.TrimSpace(plugin.ReviewStatus)
		plugin.ReviewedAt = strings.TrimSpace(plugin.ReviewedAt)
		plugin.ReviewedBy = strings.TrimSpace(plugin.ReviewedBy)
		plugin.ManualAction = strings.TrimSpace(plugin.ManualAction)
		plugin.ManifestPath = strings.TrimSpace(plugin.ManifestPath)
		if plugin.PluginID == "" || plugin.ReasonCode == "" || plugin.Summary == "" || plugin.ReviewID == "" ||
			!oneOf(plugin.ReviewStatus, "pending", "confirmed") ||
			(plugin.ReviewedAt != "" && !validRecoveryDateTime(plugin.ReviewedAt)) {
			return false
		}
	}
	if len(summary.Audit) > 50 {
		return false
	}
	for entryIndex := range summary.Audit {
		entry := &summary.Audit[entryIndex]
		entry.TaskID = strings.TrimSpace(entry.TaskID)
		entry.CreatedAt = strings.TrimSpace(entry.CreatedAt)
		entry.OperatorID = strings.TrimSpace(entry.OperatorID)
		if entry.TaskID == "" || !validRecoveryDateTime(entry.CreatedAt) || entry.OperatorID == "" || entry.Note == nil || entry.Items == nil {
			return false
		}
		note := strings.TrimSpace(*entry.Note)
		if utf8.RuneCountInString(note) > 500 {
			return false
		}
		entry.Note = &note
		for itemIndex := range *entry.Items {
			item := &(*entry.Items)[itemIndex]
			item.ReviewID = strings.TrimSpace(item.ReviewID)
			item.PluginID = strings.TrimSpace(item.PluginID)
			item.ReasonCode = strings.TrimSpace(item.ReasonCode)
			item.Summary = strings.TrimSpace(item.Summary)
			item.Version = strings.TrimSpace(item.Version)
			if item.ReviewID == "" || item.PluginID == "" || item.ReasonCode == "" || item.Summary == "" {
				return false
			}
		}
	}
	return true
}

func containsJSONNull(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case []any:
		for _, item := range typed {
			if containsJSONNull(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsJSONNull(item) {
				return true
			}
		}
	}
	return false
}

func validRecoveryDateTime(value string) bool {
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func normalizedNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
