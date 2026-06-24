package recovery

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

const (
	BackupManifestVersion = "1"
	RecoverySummaryPath   = "logs/recovery-summary.json"
	defaultCoreVersion    = "0.0.0-dev"
	reviewStatusPending   = "pending"
	reviewStatusConfirmed = "confirmed"
	maxAuditEntries       = 50
)

type BackupManifest struct {
	Version             string                    `json:"version"`
	CreatedAt           string                    `json:"created_at"`
	CoreVersion         string                    `json:"core_version"`
	ConfigSchemaVersion string                    `json:"config_schema_version"`
	DBSchemaVersion     string                    `json:"db_schema_version"`
	Consistency         string                    `json:"consistency"`
	Plugins             []BackupManifestPlugin    `json:"plugins,omitempty"`
	Directories         []BackupManifestDirectory `json:"directories,omitempty"`
}

type BackupManifestPlugin struct {
	PluginID          string   `json:"plugin_id"`
	Version           string   `json:"version,omitempty"`
	MinCoreVersion    string   `json:"min_core_version,omitempty"`
	DataSchemaVersion string   `json:"data_schema_version,omitempty"`
	Platforms         []string `json:"platforms,omitempty"`
	SourceRoot        string   `json:"source_root,omitempty"`
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
