package plugins

import (
	"encoding/json"
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
		if shouldSkipPluginDiscoveryDir(dirEntry.Name()) {
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

	defaultConfig, defaultConfigErr := manifestDefaultConfig(manifest, filepath.Dir(infoPath))

	snapshot := Snapshot{
		PluginID:           pluginID,
		Name:               stringField(manifest, "name"),
		Role:               manifestRole(manifest, sourceRoot),
		Version:            stringField(manifest, "version"),
		Author:             stringField(manifest, "author"),
		License:            stringField(manifest, "license"),
		SDKMinVersion:      stringField(manifest, "sdk_min_version"),
		RuntimeVersion:     stringField(manifest, "runtime_version"),
		MinCoreVersion:     stringField(manifest, "min_core_version"),
		DataSchemaVersion:  stringField(manifest, "data_schema_version"),
		Concurrency:        manifestConcurrency(manifest),
		Platforms:          stringListField(manifest, "platforms"),
		Type:               stringField(manifest, "type"),
		Runtime:            stringField(manifest, "runtime"),
		Entry:              stringField(manifest, "entry"),
		Description:        stringField(manifest, "description"),
		Icon:               stringField(manifest, "icon"),
		Repo:               stringField(manifest, "repo"),
		Homepage:           stringField(manifest, "homepage"),
		Keywords:           stringListField(manifest, "keywords"),
		Screenshots:        manifestScreenshots(manifest),
		ManagementUI:       manifestManagementUI(manifest),
		RenderTemplates:    manifestRenderTemplates(manifest),
		Help:               manifestHelp(manifest),
		SystemDependencies: stringListField(manifest, "system_dependencies"),
		DefaultConfig:      defaultConfig,
		ManifestPath:       displayPath(repoRoot, infoPath),
		PackageRootPath:    filepath.Dir(infoPath),
		SourceRoot:         sourceRoot,
		SourceRoots:        []string{sourceRoot},
		RegistrationState:  stateInstalled,
		DesiredState:       defaultDesiredStateForSourceRoot(sourceRoot),
		RuntimeState:       stateStopped,
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
	snapshot.ManifestCommands = manifestCommands(manifest)
	snapshot.DynamicCommands = manifestDynamicCommands(manifest)
	snapshot.Commands = ProjectCommands(snapshot, snapshot.DefaultConfig)

	if defaultConfigErr != nil {
		snapshot.Valid = false
		snapshot.DisplayState = displayInvalid
		snapshot.ValidationSummary = trimSummary(defaultConfigErr.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

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
		PackageRootPath:   "",
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

func shouldSkipPluginDiscoveryDir(name string) bool {
	switch strings.TrimSpace(name) {
	case "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache":
		return true
	default:
		return false
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

func manifestScreenshots(document map[string]any) []Screenshot {
	values, ok := document["screenshots"].([]any)
	if !ok {
		return nil
	}

	items := make([]Screenshot, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		path := stringField(item, "path")
		if path == "" {
			continue
		}
		items = append(items, Screenshot{
			Path: path,
			Alt:  stringField(item, "alt"),
		})
	}
	return items
}

func manifestManagementUI(document map[string]any) *ManagementUI {
	value, ok := document["management_ui"].(map[string]any)
	if !ok {
		return nil
	}

	entry := stringField(value, "entry")
	if entry == "" {
		return nil
	}

	return &ManagementUI{
		Entry: entry,
		Label: stringField(value, "label"),
	}
}

func manifestRenderTemplates(document map[string]any) []RenderTemplate {
	values, ok := document["render_templates"].([]any)
	if !ok {
		return nil
	}

	items := make([]RenderTemplate, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		path := stringField(item, "path")
		if path == "" {
			continue
		}
		items = append(items, RenderTemplate{Path: path})
	}
	return items
}

func manifestHelp(document map[string]any) *Help {
	value, ok := document["help"].(map[string]any)
	if !ok {
		return nil
	}

	help := &Help{
		Title:   stringField(value, "title"),
		Summary: stringField(value, "summary"),
	}
	groups, ok := value["groups"].([]any)
	if !ok {
		return help
	}

	for _, rawGroup := range groups {
		groupMap, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		group := HelpGroup{
			Title: stringField(groupMap, "title"),
		}
		rawItems, ok := groupMap["items"].([]any)
		if !ok {
			continue
		}
		for _, rawItem := range rawItems {
			itemMap, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			title := stringField(itemMap, "title")
			if title == "" {
				continue
			}
			group.Items = append(group.Items, HelpItem{
				Title:       title,
				Description: stringField(itemMap, "description"),
				Usage:       stringField(itemMap, "usage"),
				Command:     stringField(itemMap, "command"),
				Permission:  stringField(itemMap, "permission"),
			})
		}
		if group.Title != "" && len(group.Items) > 0 {
			help.Groups = append(help.Groups, group)
		}
	}
	return help
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
			Name:          name,
			Aliases:       stringListField(item, "aliases"),
			Description:   stringField(item, "description"),
			Usage:         stringField(item, "usage"),
			Permission:    stringField(item, "permission"),
			CommandSource: CommandSourceManifest,
		}
		commands = append(commands, command)
	}
	return commands
}

func manifestDynamicCommands(document map[string]any) []DynamicCommandDecl {
	values, ok := document["dynamic_commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]DynamicCommandDecl, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(item, "id")
		settingsKey := stringField(item, "settings_key")
		if id == "" || settingsKey == "" {
			continue
		}
		commands = append(commands, DynamicCommandDecl{
			ID:          id,
			SettingsKey: settingsKey,
			Description: stringField(item, "description"),
			UsageArgs:   stringField(item, "usage_args"),
			Permission:  stringField(item, "permission"),
		})
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

func manifestDefaultConfig(document map[string]any, packageRoot string) (map[string]any, error) {
	fileConfig, err := manifestDefaultConfigFile(document, packageRoot)
	if err != nil {
		return nil, err
	}

	inlineConfig := manifestObjectField(document, "default_config")
	if len(fileConfig) == 0 {
		return inlineConfig, nil
	}
	if len(inlineConfig) == 0 {
		return fileConfig, nil
	}

	merged := cloneMap(fileConfig)
	for key, value := range inlineConfig {
		merged[key] = cloneValue(value)
	}
	return merged, nil
}

func manifestDefaultConfigFile(document map[string]any, packageRoot string) (map[string]any, error) {
	relativePath := stringField(document, "default_config_file")
	if relativePath == "" {
		return nil, nil
	}
	if filepath.IsAbs(relativePath) {
		return nil, fmt.Errorf("default_config_file must be package-relative")
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}
	if filepath.Ext(cleanRelative) != ".json" {
		return nil, fmt.Errorf("default_config_file must point to a .json file")
	}

	packageRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin package root: %w", err)
	}
	configPath := filepath.Join(packageRoot, cleanRelative)
	if !pathWithinRoot(packageRoot, configPath) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read default_config_file %s: %w", relativePath, err)
	}

	var value any
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, fmt.Errorf("parse default_config_file %s: %w", relativePath, err)
	}

	config, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("default_config_file %s must contain a JSON object", relativePath)
	}
	return cloneMap(config), nil
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

func pathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (relativePath != "" && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}
