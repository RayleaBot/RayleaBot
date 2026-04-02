package recovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/storage"
)

const (
	BackupManifestVersion = "1"
	RecoverySummaryPath   = "logs/recovery-summary.json"
	defaultCoreVersion    = "0.0.0-dev"
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
	PluginID      string `json:"plugin_id"`
	Version       string `json:"version,omitempty"`
	ReasonCode    string `json:"reason_code"`
	Summary       string `json:"summary"`
	ManualAction  string `json:"manual_action,omitempty"`
	ManifestPath  string `json:"manifest_path,omitempty"`
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

func BuildBackupManifest(repoRoot string, consistency string) BackupManifest {
	return BackupManifest{
		Version:             BackupManifestVersion,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		CoreVersion:         DetectCoreVersion(repoRoot),
		ConfigSchemaVersion: config.CurrentSchemaVersion(),
		DBSchemaVersion:     storage.CurrentSchemaVersion(),
		Consistency:         strings.TrimSpace(consistency),
		Plugins:             loadManifestPlugins(filepath.Join(repoRoot, "plugins", "installed")),
	}
}

func DetectCoreVersion(repoRoot string) string {
	buildInfoPath := filepath.Join(repoRoot, "build_info.json")
	payload, err := os.ReadFile(buildInfoPath)
	if err != nil {
		return defaultCoreVersion
	}
	var buildInfo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(payload, &buildInfo); err != nil {
		return defaultCoreVersion
	}
	if strings.TrimSpace(buildInfo.Version) == "" {
		return defaultCoreVersion
	}
	return strings.TrimSpace(buildInfo.Version)
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
	summary.NextSteps = nil

	if !input.Readiness.RuntimeReady {
		summary.Status = "degraded"
		for _, issue := range input.Readiness.RuntimeIssues {
			summary.Issues = append(summary.Issues, CompatibilityIssue{
				Code:        issue.Code,
				Severity:    issue.Severity,
				Summary:     issue.Summary,
				Remediation: issue.Remediation,
			})
		}
	}

	platformName := currentPlatform()
	for _, plugin := range input.Plugins {
		if plugin.SourceRoot == "plugins/builtin" || plugin.RegistrationState != "installed" {
			continue
		}
		reasonCode, issue, skipped := pluginCompatibilityIssue(plugin, summary.TargetCoreVersion, platformName)
		if reasonCode == "" {
			continue
		}
		if summary.Status == "pending" || summary.Status == "" {
			summary.Status = "degraded"
		}
		summary.Issues = append(summary.Issues, issue)
		summary.SkippedPlugins = append(summary.SkippedPlugins, skipped)
		if input.DesiredStateRepo != nil && plugin.DesiredState != "disabled" {
			_ = input.DesiredStateRepo.SaveDesiredState(context.Background(), plugin.PluginID, "disabled", time.Now().UTC())
		}
	}

	if summary.Status == "pending" || summary.Status == "" {
		summary.Status = "compatible"
	}
	if len(summary.SkippedPlugins) > 0 {
		summary.ManualActions = append(summary.ManualActions, "处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。")
	}
	return summary
}

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

func Directory(path, label string) BackupManifestDirectory {
	return BackupManifestDirectory{Label: label, Path: filepath.ToSlash(path)}
}

func loadManifestPlugins(pluginsRoot string) []BackupManifestPlugin {
	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return nil
	}
	items := make([]BackupManifestPlugin, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		infoPath := filepath.Join(pluginsRoot, entry.Name(), "info.json")
		payload, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			continue
		}
		item := BackupManifestPlugin{
			PluginID:          stringValue(raw["id"]),
			Version:           stringValue(raw["version"]),
			MinCoreVersion:    stringValue(raw["min_core_version"]),
			DataSchemaVersion: stringValue(raw["data_schema_version"]),
			Platforms:         stringSlice(raw["platforms"]),
			SourceRoot:        "plugins/installed",
		}
		if item.PluginID == "" {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].PluginID < items[j].PluginID
	})
	return items
}

func classifyOperation(sourceVersion, targetVersion string) string {
	switch compareSemver(sourceVersion, targetVersion) {
	case -1:
		return "upgrade"
	case 1:
		return "rollback"
	default:
		return "restore"
	}
}

func compareSemver(left, right string) int {
	lp := semverParts(left)
	rp := semverParts(right)
	for i := 0; i < 3; i++ {
		if lp[i] < rp[i] {
			return -1
		}
		if lp[i] > rp[i] {
			return 1
		}
	}
	return 0
}

func semverParts(version string) [3]int {
	var parts [3]int
	if version == "" {
		return parts
	}
	cleaned := version
	for _, marker := range []string{"-", "+"} {
		if idx := strings.Index(cleaned, marker); idx >= 0 {
			cleaned = cleaned[:idx]
		}
	}
	items := strings.Split(cleaned, ".")
	for i := 0; i < len(items) && i < 3; i++ {
		value, err := strconv.Atoi(strings.TrimSpace(items[i]))
		if err == nil {
			parts[i] = value
		}
	}
	return parts
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
	return compareSemver(source, target) > 0
}

func currentPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "windows-x64"
	case "linux":
		return "linux-x64"
	case "darwin":
		return "macos-arm64"
	default:
		return runtime.GOOS
	}
}

func pluginCompatibilityIssue(plugin plugins.Snapshot, targetCoreVersion, platformName string) (string, CompatibilityIssue, SkippedPlugin) {
	if strings.TrimSpace(plugin.MinCoreVersion) != "" && compareSemver(plugin.MinCoreVersion, targetCoreVersion) > 0 {
		return "plugin.min_core_version", CompatibilityIssue{
				Code:        "recovery.plugin_min_core_version",
				Severity:    "warning",
				Summary:     fmt.Sprintf("插件 %s 需要更高版本的 RayleaBot core。", plugin.PluginID),
				Remediation: "升级程序或安装与当前版本兼容的插件包后，再手动重新启用该插件。",
			}, SkippedPlugin{
				PluginID:     plugin.PluginID,
				Version:      plugin.Version,
				ReasonCode:   "plugin.min_core_version",
				Summary:      "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
				ManualAction: "升级程序或重新安装兼容版本插件。",
				ManifestPath: plugin.ManifestPath,
			}
	}
	if len(plugin.Platforms) > 0 && !contains(plugin.Platforms, platformName) {
		return "plugin.platform_mismatch", CompatibilityIssue{
				Code:        "recovery.plugin_platform_mismatch",
				Severity:    "warning",
				Summary:     fmt.Sprintf("插件 %s 不支持当前运行平台。", plugin.PluginID),
				Remediation: "请改用支持当前平台的插件包后，再手动重新启用该插件。",
			}, SkippedPlugin{
				PluginID:     plugin.PluginID,
				Version:      plugin.Version,
				ReasonCode:   "plugin.platform_mismatch",
				Summary:      "插件平台兼容性不满足，已保留安装目录并跳过自动启用。",
				ManualAction: "安装支持当前平台的插件包。",
				ManifestPath: plugin.ManifestPath,
			}
	}
	return "", CompatibilityIssue{}, SkippedPlugin{}
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

func ReadDBSchemaVersion(ctx context.Context, databasePath string) string {
	if strings.TrimSpace(databasePath) == "" {
		return ""
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return ""
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		return ""
	}
	return fmt.Sprintf("%04d", count)
}

func EmbeddedMigrationCount() string {
	return storage.CurrentSchemaVersion()
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

func ScanRepoPaths(repoRoot, configPath, databasePath string) []BackupManifestDirectory {
	items := make([]BackupManifestDirectory, 0, 3)
	if configPath != "" {
		if relative, err := filepath.Rel(repoRoot, configPath); err == nil {
			items = append(items, Directory(relative, "config"))
		}
	}
	if databasePath != "" {
		if relative, err := filepath.Rel(repoRoot, databasePath); err == nil {
			items = append(items, Directory(relative, "database"))
		}
	}
	pluginsPath := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(pluginsPath); err == nil && info.IsDir() {
		if relative, err := filepath.Rel(repoRoot, pluginsPath); err == nil {
			items = append(items, Directory(relative, "plugins"))
		}
	}
	return items
}

func RepoRootFromConfigPath(configPath string) string {
	return filepath.Dir(filepath.Dir(configPath))
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
