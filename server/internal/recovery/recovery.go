package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	semverutil "github.com/RayleaBot/RayleaBot/server/internal/semver"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const (
	BackupManifestVersion = "3"
	PluginManifestVersion = "3"
	PluginProtocolVersion = "2"
	PluginUIBridgeVersion = "3"
	PluginArtifactVersion = "2"
	RecoverySummaryPath   = "logs/recovery-summary.json"
	defaultCoreVersion    = "0.4.0"
	reviewStatusPending   = "pending"
	reviewStatusConfirmed = "confirmed"
	maxAuditEntries       = 50
)

type BackupManifest struct {
	Version               string                    `json:"version"`
	CreatedAt             string                    `json:"created_at"`
	CoreVersion           string                    `json:"core_version"`
	ConfigSchemaVersion   string                    `json:"config_schema_version"`
	DBSchemaVersion       string                    `json:"db_schema_version"`
	PluginManifestVersion string                    `json:"plugin_manifest_version"`
	PluginProtocolVersion string                    `json:"plugin_protocol_version"`
	PluginArtifactVersion string                    `json:"plugin_artifact_version"`
	PluginUIBridgeVersion string                    `json:"plugin_ui_bridge_version"`
	Consistency           string                    `json:"consistency"`
	Plugins               []BackupManifestPlugin    `json:"plugins,omitempty"`
	Directories           []BackupManifestDirectory `json:"directories,omitempty"`
}

type BackupManifestPlugin struct {
	PluginID        string `json:"plugin_id"`
	ManifestVersion string `json:"manifest_version"`
	ProtocolVersion string `json:"protocol_version"`
	ArtifactVersion string `json:"artifact_version"`
	Version         string `json:"version,omitempty"`
	MinCoreVersion  string `json:"min_core_version,omitempty"`
	SourceRoot      string `json:"source_root,omitempty"`
}

type BackupManifestDirectory struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type CompatibilityIssue struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation,omitempty"`
}

type SkippedPlugin struct {
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

type AuditItem struct {
	ReviewID   string `json:"review_id"`
	PluginID   string `json:"plugin_id"`
	ReasonCode string `json:"reason_code"`
	Summary    string `json:"summary"`
	Version    string `json:"version,omitempty"`
}

type AuditEntry struct {
	TaskID     string      `json:"task_id"`
	CreatedAt  string      `json:"created_at"`
	OperatorID string      `json:"operator_id"`
	Note       string      `json:"note"`
	Items      []AuditItem `json:"items"`
}

type CompatibilitySummary struct {
	Status                    string               `json:"status"`
	Phase                     string               `json:"phase"`
	Operation                 string               `json:"operation"`
	CreatedAt                 string               `json:"created_at"`
	UpdatedAt                 string               `json:"updated_at"`
	SourceCoreVersion         string               `json:"source_core_version,omitempty"`
	TargetCoreVersion         string               `json:"target_core_version,omitempty"`
	SourceConfigSchemaVersion string               `json:"source_config_schema_version,omitempty"`
	TargetConfigSchemaVersion string               `json:"target_config_schema_version,omitempty"`
	SourceDBSchemaVersion     string               `json:"source_db_schema_version,omitempty"`
	TargetDBSchemaVersion     string               `json:"target_db_schema_version,omitempty"`
	RequiresPostStartChecks   bool                 `json:"requires_post_start_checks,omitempty"`
	Issues                    []CompatibilityIssue `json:"issues,omitempty"`
	SkippedPlugins            []SkippedPlugin      `json:"skipped_plugins,omitempty"`
	ManualActions             []string             `json:"manual_actions,omitempty"`
	NextSteps                 []string             `json:"next_steps,omitempty"`
	Audit                     []AuditEntry         `json:"audit,omitempty"`
}

type RuntimeReadiness struct {
	RuntimeReady  bool
	RuntimeIssues []CompatibilityIssue
}

type FinalizeInput struct {
	Plugins          []plugins.Snapshot
	DesiredStateRepo plugins.DesiredStateRepository
	Readiness        RuntimeReadiness
}

type UnknownReviewIDsError struct {
	ReviewIDs []string
}

func (e *UnknownReviewIDsError) Error() string {
	return "unknown recovery review ids"
}

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

	if manifest.Version != BackupManifestVersion ||
		manifest.PluginManifestVersion != PluginManifestVersion ||
		manifest.PluginProtocolVersion != PluginProtocolVersion ||
		manifest.PluginArtifactVersion != PluginArtifactVersion ||
		manifest.PluginUIBridgeVersion != PluginUIBridgeVersion {
		summary.Status = "blocked"
		summary.RequiresPostStartChecks = false
		summary.Issues = append(summary.Issues, CompatibilityIssue{
			Code:        "plugin.contract_unsupported",
			Severity:    "error",
			Summary:     "备份清单合同版本不受支持，不能恢复。",
			Remediation: "请使用 backup manifest v3 重新创建备份。",
		})
	} else {
		for _, plugin := range manifest.Plugins {
			if plugin.ManifestVersion != PluginManifestVersion || plugin.ProtocolVersion != PluginProtocolVersion || plugin.ArtifactVersion != PluginArtifactVersion {
				summary.Issues = append(summary.Issues, CompatibilityIssue{
					Code:        "plugin.contract_unsupported",
					Severity:    "warning",
					Summary:     fmt.Sprintf("插件 %s 使用旧合同版本，恢复后会保留数据并保持无效状态。", plugin.PluginID),
					Remediation: "恢复完成后安装该插件的 manifest v3、protocol v2、artifact v2 版本。",
				})
			}
		}
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

	for _, plugin := range input.Plugins {
		if plugin.RegistrationState != "installed" {
			continue
		}
		reasonCode, skipped := pluginCompatibilityIssue(plugin, summary.TargetCoreVersion)
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

func ConfirmSkippedPlugins(summary CompatibilitySummary, reviewIDs []string, operatorID, note, taskID string) (CompatibilitySummary, []string, error) {
	reviewIDs = dedupeStrings(reviewIDs)
	if len(reviewIDs) == 0 {
		return summary, nil, &UnknownReviewIDsError{}
	}

	indexByReviewID := map[string]int{}
	for index, skipped := range summary.SkippedPlugins {
		if strings.TrimSpace(skipped.ReviewID) != "" {
			indexByReviewID[skipped.ReviewID] = index
		}
	}

	unknown := make([]string, 0, len(reviewIDs))
	for _, reviewID := range reviewIDs {
		if _, ok := indexByReviewID[reviewID]; !ok {
			unknown = append(unknown, reviewID)
		}
	}
	if len(unknown) > 0 {
		return summary, nil, &UnknownReviewIDsError{ReviewIDs: unknown}
	}

	operatorID = strings.TrimSpace(operatorID)
	note = strings.TrimSpace(note)
	confirmedAt := time.Now().UTC().Format(time.RFC3339)
	newlyConfirmed := make([]string, 0, len(reviewIDs))
	auditItems := make([]AuditItem, 0, len(reviewIDs))

	for _, reviewID := range reviewIDs {
		skipped := &summary.SkippedPlugins[indexByReviewID[reviewID]]
		if skipped.ReviewStatus == reviewStatusConfirmed {
			continue
		}
		skipped.ReviewStatus = reviewStatusConfirmed
		skipped.ReviewedAt = confirmedAt
		skipped.ReviewedBy = operatorID
		newlyConfirmed = append(newlyConfirmed, reviewID)
		auditItems = append(auditItems, AuditItem{
			ReviewID:   skipped.ReviewID,
			PluginID:   skipped.PluginID,
			ReasonCode: skipped.ReasonCode,
			Summary:    skipped.Summary,
			Version:    skipped.Version,
		})
	}

	machineIssues := filterMachineIssues(summary.Issues)
	pendingSkipped := pendingSkippedPlugins(summary.SkippedPlugins)
	summary.Issues = append(machineIssues, issuesForSkippedPlugins(pendingSkipped)...)
	if len(summary.Issues) == 0 {
		summary.Issues = nil
	}
	summary.Status = recoveryStatus(machineIssues, pendingSkipped)
	if summary.Status == "compatible" {
		summary.ManualActions = nil
		summary.NextSteps = nil
	} else {
		summary.ManualActions = buildManualActions(machineIssues, pendingSkipped)
		summary.NextSteps = buildNextSteps(machineIssues, pendingSkipped)
	}
	if len(newlyConfirmed) > 0 {
		summary.UpdatedAt = confirmedAt
		summary.Audit = trimAuditEntries(append([]AuditEntry{{
			TaskID:     strings.TrimSpace(taskID),
			CreatedAt:  confirmedAt,
			OperatorID: operatorID,
			Note:       note,
			Items:      auditItems,
		}}, summary.Audit...))
	}
	return summary, newlyConfirmed, nil
}

type reviewConfirmation struct {
	ReviewedAt string
	ReviewedBy string
}

func confirmedReviewLookup(summary CompatibilitySummary) map[string]reviewConfirmation {
	lookup := map[string]reviewConfirmation{}
	for _, skipped := range summary.SkippedPlugins {
		if skipped.ReviewStatus != reviewStatusConfirmed || strings.TrimSpace(skipped.ReviewID) == "" {
			continue
		}
		lookup[skipped.ReviewID] = reviewConfirmation{
			ReviewedAt: skipped.ReviewedAt,
			ReviewedBy: skipped.ReviewedBy,
		}
	}
	for _, entry := range summary.Audit {
		for _, item := range entry.Items {
			if strings.TrimSpace(item.ReviewID) == "" {
				continue
			}
			if _, exists := lookup[item.ReviewID]; exists {
				continue
			}
			lookup[item.ReviewID] = reviewConfirmation{
				ReviewedAt: entry.CreatedAt,
				ReviewedBy: entry.OperatorID,
			}
		}
	}
	return lookup
}

func classifyOperation(sourceVersion, targetVersion string) string {
	switch semverutil.Compare(sourceVersion, targetVersion) {
	case -1:
		return "upgrade"
	case 1:
		return "rollback"
	default:
		return "restore"
	}
}

func isSchemaNewer(source, target string) bool {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" || target == "" {
		return false
	}
	left, leftErr := strconv.Atoi(source)
	right, rightErr := strconv.Atoi(target)
	if leftErr == nil && rightErr == nil {
		return left > right
	}
	leftBase, leftBaseOK := baseSchemaVersion(source)
	rightBase, rightBaseOK := baseSchemaVersion(target)
	if leftBaseOK && rightBaseOK {
		return leftBase > rightBase
	}
	if leftBaseOK != rightBaseOK || leftErr != nil || rightErr != nil {
		return false
	}
	return semverutil.Compare(source, target) > 0
}

func baseSchemaVersion(version string) (int, bool) {
	const prefix = "base-"
	if !strings.HasPrefix(version, prefix) {
		return 0, false
	}
	parts := strings.Split(strings.TrimPrefix(version, prefix), "-")
	if len(parts) != 2 {
		return 0, false
	}
	year, yearErr := strconv.Atoi(parts[0])
	month, monthErr := strconv.Atoi(parts[1])
	if yearErr != nil || monthErr != nil || month < 1 || month > 12 {
		return 0, false
	}
	return year*100 + month, true
}

func pluginCompatibilityIssue(plugin plugins.Snapshot, targetCoreVersion string) (string, SkippedPlugin) {
	if plugin.ManifestVersion != PluginManifestVersion || plugin.ArtifactVersion != PluginArtifactVersion {
		return "plugin.contract_unsupported", SkippedPlugin{
			PluginID: plugin.PluginID, Version: plugin.Version,
			ReasonCode:   "plugin.contract_unsupported",
			Summary:      "插件合同版本不受支持，已保留安装目录和插件数据并跳过自动启用。",
			ReviewID:     buildReviewID(plugin.PluginID, "plugin.contract_unsupported", plugin.Version),
			ReviewStatus: reviewStatusPending,
			ManualAction: "安装 manifest v3、protocol v2、artifact v2 插件包。",
			ManifestPath: plugin.ManifestPath,
		}
	}
	if strings.TrimSpace(plugin.MinCoreVersion) != "" && semverutil.Compare(plugin.MinCoreVersion, targetCoreVersion) > 0 {
		return "plugin.min_core_version", SkippedPlugin{
			PluginID:     plugin.PluginID,
			Version:      plugin.Version,
			ReasonCode:   "plugin.min_core_version",
			Summary:      "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
			ReviewID:     buildReviewID(plugin.PluginID, "plugin.min_core_version", plugin.Version),
			ReviewStatus: reviewStatusPending,
			ManualAction: "升级程序或重新安装兼容版本插件。",
			ManifestPath: plugin.ManifestPath,
		}
	}
	return "", SkippedPlugin{}
}

func buildReviewID(pluginID, reasonCode, version string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(pluginID),
		strings.TrimSpace(reasonCode),
		strings.TrimSpace(version),
	}, "\x00")))
	return "review_" + hex.EncodeToString(sum[:])
}

func pluginIssueFromSkipped(skipped SkippedPlugin) CompatibilityIssue {
	switch strings.TrimSpace(skipped.ReasonCode) {
	case "plugin.contract_unsupported":
		return CompatibilityIssue{
			Code: "plugin.contract_unsupported", Severity: "warning",
			Summary:     fmt.Sprintf("插件 %s 的合同版本不受支持。", skipped.PluginID),
			Remediation: "安装当前插件合同版本后再手动启用；现有设置、密钥、KV、文件和公开数据不会被删除。",
		}
	case "plugin.min_core_version":
		return CompatibilityIssue{
			Code:        "recovery.plugin_min_core_version",
			Severity:    "warning",
			Summary:     fmt.Sprintf("插件 %s 需要更高版本的 RayleaBot core。", skipped.PluginID),
			Remediation: "升级程序或安装与当前版本兼容的插件包后，再手动重新启用该插件。",
		}
	case "plugin.platform_mismatch":
		return CompatibilityIssue{
			Code:        "recovery.plugin_platform_mismatch",
			Severity:    "warning",
			Summary:     fmt.Sprintf("插件 %s 不支持当前运行平台。", skipped.PluginID),
			Remediation: "请改用支持当前平台的插件包后，再手动重新启用该插件。",
		}
	default:
		return CompatibilityIssue{
			Code:        "recovery.plugin_incompatible",
			Severity:    "warning",
			Summary:     skipped.Summary,
			Remediation: skipped.ManualAction,
		}
	}
}
