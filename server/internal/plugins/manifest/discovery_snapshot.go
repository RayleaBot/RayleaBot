package pluginmanifest

import (
	"log/slog"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func LoadSnapshot(infoPath, sourceRoot, repoRoot string, validator *schema.Validator, maxSummaryChars int, logger *slog.Logger) (Snapshot, bool, error) {
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
		RegistrationState:  RegistrationStateInstalled,
		DesiredState:       defaultDesiredStateForSourceRoot(sourceRoot),
		RuntimeState:       RuntimeStateStopped,
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
		snapshot.DisplayState = DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(defaultConfigErr.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	if err := validator.Validate(document); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	if err := validateManagementUIPages(snapshot.ManagementUI); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	snapshot.Valid = true
	snapshot.DisplayState = DisplayStateDiscovered
	return snapshot, true, nil
}
