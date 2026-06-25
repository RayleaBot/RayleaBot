package model

import (
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type StatusSnapshot struct {
	Status          string
	AdapterState    string
	ActivePlugins   int
	RunningPlugins  int
	FailedPlugins   int
	DBSchemaVersion string
	UptimeSeconds   int64
	RecoverySummary *recovery.CompatibilitySummary
	Health          *health.ReadinessReport
}

type DiagnosticsSnapshot struct {
	GeneratedAt     string                         `json:"generated_at"`
	Build           DiagnosticsBuild               `json:"build"`
	System          DiagnosticsSystem              `json:"system"`
	Config          DiagnosticsConfig              `json:"config"`
	Secrets         DiagnosticsSecrets             `json:"secrets"`
	Database        DiagnosticsDatabase            `json:"database"`
	Adapter         DiagnosticsAdapter             `json:"adapter"`
	Plugins         DiagnosticsPlugins             `json:"plugins"`
	Render          DiagnosticsIssueGroup          `json:"render"`
	ThirdParty      DiagnosticsThirdParty          `json:"third_party"`
	BilibiliSource  DiagnosticsBilibiliSource      `json:"bilibili_source"`
	Scheduler       DiagnosticsScheduler           `json:"scheduler"`
	Tasks           DiagnosticsTaskSummary         `json:"tasks"`
	Dependencies    []DiagnosticsDependency        `json:"dependencies"`
	Filesystem      []DiagnosticsPathPermission    `json:"filesystem"`
	RecentErrors    []logging.Summary              `json:"recent_errors"`
	Issues          []health.DiagnosticIssue       `json:"issues"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

type DiagnosticsBuild struct {
	CoreVersion string `json:"core_version"`
}

type DiagnosticsSystem struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type DiagnosticsConfig struct {
	SchemaVersion    string `json:"schema_version"`
	Status           string `json:"status"`
	ApplyState       string `json:"apply_state"`
	ConfigPath       string `json:"config_path"`
	SchemaPath       string `json:"schema_path"`
	DatabaseEngine   string `json:"database_engine"`
	DatabasePath     string `json:"database_path"`
	OneBotConfigured bool   `json:"onebot_configured"`
}

type DiagnosticsSecrets struct {
	UnresolvedRefs []string `json:"unresolved_refs"`
}

type DiagnosticsDatabase struct {
	SchemaVersion     string                 `json:"schema_version"`
	AppliedMigrations []DiagnosticsMigration `json:"applied_migrations"`
}

type DiagnosticsMigration struct {
	Version   string `json:"version"`
	Name      string `json:"name"`
	AppliedAt string `json:"applied_at"`
}

type DiagnosticsAdapter struct {
	State string `json:"state"`
}

type DiagnosticsPlugins struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
}

type DiagnosticsIssueGroup struct {
	Status string                   `json:"status"`
	Issues []health.DiagnosticIssue `json:"issues"`
}

type DiagnosticsThirdParty struct {
	Total      int                             `json:"total"`
	Enabled    int                             `json:"enabled"`
	Configured int                             `json:"configured"`
	Invalid    int                             `json:"invalid"`
	Platforms  []DiagnosticsThirdPartyPlatform `json:"platforms"`
}

type DiagnosticsThirdPartyPlatform struct {
	Platform   string `json:"platform"`
	Total      int    `json:"total"`
	Enabled    int    `json:"enabled"`
	Configured int    `json:"configured"`
	Invalid    int    `json:"invalid"`
}

type DiagnosticsBilibiliSource struct {
	Status            string                   `json:"status"`
	Summary           string                   `json:"summary"`
	DiagnosisLevel    string                   `json:"diagnosis_level"`
	WatchedRooms      int                      `json:"watched_rooms"`
	WatchedUIDs       int                      `json:"watched_uids"`
	LiveLastEventAt   string                   `json:"live_last_event_at,omitempty"`
	DynamicLastPollAt string                   `json:"dynamic_last_poll_at,omitempty"`
	Issues            []health.DiagnosticIssue `json:"issues"`
}

type DiagnosticsScheduler struct {
	Total    int `json:"total"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
	Pending  int `json:"pending"`
	Running  int `json:"running"`
	Failed   int `json:"failed"`
}

type DiagnosticsTaskSummary struct {
	Pending int `json:"pending"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
}

type DiagnosticsDependency struct {
	Kind                 string `json:"kind"`
	Status               string `json:"status"`
	MetadataComplete     bool   `json:"metadata_complete"`
	CachedArchivePresent bool   `json:"cached_archive_present"`
	PreparedStorePresent bool   `json:"prepared_store_present"`
	SystemBrowser        bool   `json:"system_browser"`
}

type DiagnosticsPathPermission struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Status string `json:"status"`
	IsDir  bool   `json:"is_dir"`
}

type ErrorReason string

const (
	ErrorReasonInternal        ErrorReason = "internal"
	ErrorReasonInvalidRequest  ErrorReason = "invalid_request"
	ErrorReasonResourceMissing ErrorReason = "resource_missing"
)

type Error struct {
	Reason  ErrorReason
	Details map[string]any
}

func InternalError() *Error {
	return &Error{Reason: ErrorReasonInternal}
}

func InvalidRequestError(details map[string]any) *Error {
	return &Error{Reason: ErrorReasonInvalidRequest, Details: details}
}

func ResourceMissingError(details map[string]any) *Error {
	return &Error{Reason: ErrorReasonResourceMissing, Details: details}
}

func RecoverySummaryDetails(repoRoot string) map[string]any {
	return map[string]any{
		"resource_type": "recovery_summary",
		"path":          recovery.SummaryPath(repoRoot),
	}
}
