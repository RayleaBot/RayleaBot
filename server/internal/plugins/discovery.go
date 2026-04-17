package plugins

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

const (
	stateInstalled       = "installed"
	stateRemoved         = "removed"
	stateDisabled        = "disabled"
	stateStopped         = "stopped"
	displayDiscovered    = "discovered"
	displayInvalid       = "invalid_manifest"
	displayConflict      = "conflict"
	validationMaxSummary = 256
)

type ScanRoot struct {
	Label string
	Path  string
}

type DiscoverOptions struct {
	Validator       *schema.Validator
	Roots           []ScanRoot
	RepoRoot        string
	Logger          *slog.Logger
	MaxSummaryChars int
}

type DiscoverSummary struct {
	ValidCount    int
	InvalidCount  int
	ConflictCount int
	SkippedCount  int
}

func Discover(options DiscoverOptions) ([]Snapshot, DiscoverSummary, error) {
	if options.Validator == nil {
		return nil, DiscoverSummary{}, fmt.Errorf("plugin manifest validator is required")
	}

	maxSummaryChars := options.MaxSummaryChars
	if maxSummaryChars <= 0 {
		maxSummaryChars = validationMaxSummary
	}

	var summary DiscoverSummary
	byPluginID := map[string][]Snapshot{}

	for _, root := range options.Roots {
		entries, skipped, err := discoverRoot(root, options.Validator, options.RepoRoot, maxSummaryChars, options.Logger)
		if err != nil {
			return nil, summary, err
		}

		summary.SkippedCount += skipped
		for _, entry := range entries {
			byPluginID[entry.PluginID] = append(byPluginID[entry.PluginID], entry)
		}
	}

	pluginIDs := make([]string, 0, len(byPluginID))
	for pluginID := range byPluginID {
		pluginIDs = append(pluginIDs, pluginID)
	}
	sort.Strings(pluginIDs)

	snapshots := make([]Snapshot, 0, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		group := byPluginID[pluginID]
		if len(group) == 1 {
			entry := group[0]
			if entry.Valid {
				summary.ValidCount++
				logPluginDiscovered(options.Logger, entry)
			} else {
				summary.InvalidCount++
				logPluginInvalid(options.Logger, entry)
			}
			snapshots = append(snapshots, entry)
			continue
		}

		conflictSnapshot := buildConflictSnapshot(pluginID, group)
		summary.ConflictCount++
		logPluginConflict(options.Logger, conflictSnapshot)
		snapshots = append(snapshots, conflictSnapshot)
	}

	if options.Logger != nil {
		options.Logger.Info(
			"plugin discovery complete",
			"component", "plugins",
			"valid_count", summary.ValidCount,
			"invalid_count", summary.InvalidCount,
			"conflict_count", summary.ConflictCount,
			"skipped_count", summary.SkippedCount,
		)
	}

	return snapshots, summary, nil
}

func discoverRoot(root ScanRoot, validator *schema.Validator, repoRoot string, maxSummaryChars int, logger *slog.Logger) ([]Snapshot, int, error) {
	if logger != nil {
		logger.Info(
			"plugin discovery starting",
			"component", "plugins",
			"source_root", root.Label,
		)
	}

	dirEntries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Info(
					"plugin source root missing, skipping",
					"component", "plugins",
					"source_root", root.Label,
				)
			}
			return nil, 0, nil
		}

		return nil, 0, fmt.Errorf("read plugin root %s: %w", root.Path, err)
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	var snapshots []Snapshot
	skipped := 0

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(root.Path, dirEntry.Name())
		infoPath := filepath.Join(pluginDir, "info.json")
		if _, err := os.Stat(infoPath); err != nil {
			if os.IsNotExist(err) {
				skipped++
				if logger != nil {
					logger.Warn(
						"plugin directory skipped because info.json is missing",
						"component", "plugins",
						"plugin_dir", displayPath(repoRoot, pluginDir),
						"manifest_path", displayPath(repoRoot, infoPath),
						"source_root", root.Label,
					)
				}
				continue
			}

			return nil, skipped, fmt.Errorf("stat %s: %w", infoPath, err)
		}

		snapshot, ok, err := loadSnapshot(infoPath, root.Label, repoRoot, validator, maxSummaryChars, logger)
		if err != nil {
			return nil, skipped, err
		}
		if !ok {
			skipped++
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, skipped, nil
}

func loadSnapshot(infoPath, sourceRoot, repoRoot string, validator *schema.Validator, maxSummaryChars int, logger *slog.Logger) (Snapshot, bool, error) {
	document, err := schema.LoadJSONFile(infoPath)
	if err != nil {
		if logger != nil {
			logger.Warn(
				"plugin manifest skipped because json parsing failed",
				"component", "plugins",
				"manifest_path", displayPath(repoRoot, infoPath),
				"source_root", sourceRoot,
				"err", err.Error(),
			)
		}
		return Snapshot{}, false, nil
	}

	manifest, ok := document.(map[string]any)
	if !ok {
		if logger != nil {
			logger.Warn(
				"plugin manifest skipped because the top-level document is not an object",
				"component", "plugins",
				"manifest_path", displayPath(repoRoot, infoPath),
				"source_root", sourceRoot,
			)
		}
		return Snapshot{}, false, nil
	}

	pluginID, ok := extractStringField(manifest, "id")
	if !ok {
		if logger != nil {
			logger.Warn(
				"plugin manifest skipped because id is missing or invalid",
				"component", "plugins",
				"manifest_path", displayPath(repoRoot, infoPath),
				"source_root", sourceRoot,
			)
		}
		return Snapshot{}, false, nil
	}

	snapshot := Snapshot{
		PluginID:          pluginID,
		Name:              stringField(manifest, "name"),
		Role:              manifestRole(manifest, sourceRoot),
		Version:           stringField(manifest, "version"),
		MinCoreVersion:    stringField(manifest, "min_core_version"),
		DataSchemaVersion: stringField(manifest, "data_schema_version"),
		Concurrency:       manifestConcurrency(manifest),
		Platforms:         stringListField(manifest, "platforms"),
		Type:              stringField(manifest, "type"),
		Runtime:           stringField(manifest, "runtime"),
		Entry:             stringField(manifest, "entry"),
		Description:       stringField(manifest, "description"),
		DefaultConfig:     manifestObjectField(manifest, "default_config"),
		ManifestPath:      displayPath(repoRoot, infoPath),
		SourceRoot:        sourceRoot,
		SourceRoots:       []string{sourceRoot},
		RegistrationState: stateInstalled,
		DesiredState:      defaultDesiredStateForSourceRoot(sourceRoot),
		RuntimeState:      stateStopped,
	}
	snapshot.RequiredPermissions = manifestPermissionList(manifest, "required")
	snapshot.OptionalPermissions = manifestPermissionList(manifest, "optional")
	snapshot.DeclaredCapabilities = stringListField(manifest, "capabilities")
	snapshot.PythonDependencies = manifestDependencyList(manifest, "python")
	snapshot.NodeDependencies = manifestDependencyList(manifest, "nodejs")
	snapshot.RequireInstallScripts = manifestBoolField(manifest, "require_install_scripts")
	snapshot.ScopeHTTPHosts = manifestScopeList(manifest, "http_hosts")
	snapshot.ScopeStorageRoots = manifestScopeList(manifest, "storage_roots")
	snapshot.ScopeWebhooks = manifestWebhookScopes(manifest)
	snapshot.Commands = manifestCommands(manifest)

	if err := validator.Validate(document); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = displayInvalid
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	snapshot.Valid = true
	snapshot.DisplayState = displayDiscovered
	return snapshot, true, nil
}

func buildConflictSnapshot(pluginID string, group []Snapshot) Snapshot {
	conflictPaths := make([]string, 0, len(group))
	sourceRoots := make([]string, 0, len(group))
	for _, entry := range group {
		conflictPaths = append(conflictPaths, entry.ManifestPath)
		if !containsString(sourceRoots, entry.SourceRoot) {
			sourceRoots = append(sourceRoots, entry.SourceRoot)
		}
	}

	sort.Strings(conflictPaths)
	sort.Strings(sourceRoots)

	return Snapshot{
		PluginID:          pluginID,
		ManifestPath:      "",
		SourceRoot:        "",
		SourceRoots:       sourceRoots,
		Valid:             false,
		ValidationSummary: "duplicate plugin_id discovered across multiple directories",
		RegistrationState: stateInstalled,
		DesiredState:      stateDisabled,
		RuntimeState:      stateStopped,
		DisplayState:      displayConflict,
		ConflictPaths:     conflictPaths,
	}
}

func logPluginDiscovered(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Info(
		"plugin discovered",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
	)
}

func logPluginInvalid(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"plugin manifest invalid",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
		"validation_summary", entry.ValidationSummary,
	)
}

func logPluginConflict(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"plugin id conflict",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"count", len(entry.ConflictPaths),
		"source_roots", strings.Join(entry.SourceRoots, ","),
	)
}

func extractStringField(document map[string]any, key string) (string, bool) {
	value, ok := document[key]
	if !ok {
		return "", false
	}

	stringValue, ok := value.(string)
	if !ok || stringValue == "" {
		return "", false
	}

	return stringValue, true
}

func stringField(document map[string]any, key string) string {
	value, ok := document[key]
	if !ok {
		return ""
	}

	stringValue, ok := value.(string)
	if !ok {
		return ""
	}

	return stringValue
}

func manifestBoolField(document map[string]any, key string) bool {
	value, ok := document[key]
	if !ok {
		return false
	}
	booleanValue, ok := value.(bool)
	if !ok {
		return false
	}
	return booleanValue
}

func manifestConcurrency(document map[string]any) int {
	value, ok := document["concurrency"]
	if !ok {
		return 1
	}
	switch typed := value.(type) {
	case int:
		if typed >= 1 {
			return typed
		}
	case int64:
		if typed >= 1 {
			return int(typed)
		}
	case float64:
		if typed >= 1 {
			return int(typed)
		}
	}
	return 1
}

func manifestPermissionList(document map[string]any, key string) []string {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(permissions, key)
}

func manifestDependencyList(document map[string]any, key string) []string {
	dependencies, ok := document["dependencies"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(dependencies, key)
}

func manifestScopeList(document map[string]any, key string) []string {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	scopes, ok := permissions["scopes"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(scopes, key)
}

func manifestWebhookScopes(document map[string]any) []WebhookScope {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	scopes, ok := permissions["scopes"].(map[string]any)
	if !ok {
		return nil
	}
	values, ok := scopes["webhooks"].([]any)
	if !ok {
		return nil
	}

	items := make([]WebhookScope, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		scope := WebhookScope{
			Route:           stringField(item, "route"),
			AuthStrategy:    stringField(item, "auth_strategy"),
			Header:          stringField(item, "header"),
			SecretRef:       stringField(item, "secret_ref"),
			SignaturePrefix: stringField(item, "signature_prefix"),
			SourceIPs:       stringListField(item, "source_ips"),
		}
		if scope.Route == "" || scope.AuthStrategy == "" || scope.Header == "" || scope.SecretRef == "" {
			continue
		}
		items = append(items, scope)
	}
	return items
}

func manifestCommands(document map[string]any) []Command {
	values, ok := document["commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]Command, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := stringField(item, "name")
		if name == "" {
			continue
		}
		command := Command{
			Name:        name,
			Aliases:     stringListField(item, "aliases"),
			Description: stringField(item, "description"),
			Usage:       stringField(item, "usage"),
			Permission:  stringField(item, "permission"),
		}
		commands = append(commands, command)
	}
	return commands
}

func manifestRole(document map[string]any, sourceRoot string) string {
	role := stringField(document, "role")
	if role != "" {
		return role
	}

	switch sourceRoot {
	case "plugins/builtin":
		return "builtin"
	case "examples/plugins":
		return "example"
	case "plugins/dev":
		return "dev"
	default:
		return "user"
	}
}

func manifestObjectField(document map[string]any, key string) map[string]any {
	value, ok := document[key].(map[string]any)
	if !ok {
		return nil
	}
	return cloneMap(value)
}

func defaultDesiredStateForSourceRoot(sourceRoot string) string {
	if sourceRoot == "plugins/builtin" {
		return "enabled"
	}
	return stateDisabled
}

func stringListField(document map[string]any, key string) []string {
	values, ok := document[key].([]any)
	if !ok {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		items = append(items, text)
	}
	return items
}

func trimSummary(summary string, maxLen int) string {
	singleLine := strings.Join(strings.Fields(summary), " ")
	if len(singleLine) <= maxLen {
		return singleLine
	}

	return singleLine[:maxLen-3] + "..."
}

func displayPath(repoRoot, path string) string {
	if repoRoot != "" {
		relativePath, err := filepath.Rel(repoRoot, path)
		if err == nil && relativePath != "." && !strings.HasPrefix(relativePath, "..") {
			return filepath.ToSlash(relativePath)
		}
	}

	return filepath.ToSlash(path)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}
