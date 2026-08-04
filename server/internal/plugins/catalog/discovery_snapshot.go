package catalog

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

func LoadSnapshot(infoPath, sourceRoot, repoRoot string, validator *config.Validator, maxSummaryChars int, logger *slog.Logger) (plugins.Snapshot, bool, error) {
	document, err := config.LoadJSONFile(infoPath)
	if err != nil {
		if logger != nil {
			logger.Warn(
				fmt.Sprintf("插件清单 JSON 解析失败，已跳过：%s（来源：%s）", logpath.Display(repoRoot, infoPath), sourceRoot),
				"component", "plugins",
				"manifest_path", logpath.Display(repoRoot, infoPath),
				"source_root", sourceRoot,
				"err", err.Error(),
			)
		}
		return plugins.Snapshot{}, false, nil
	}

	manifest, ok := document.(map[string]any)
	if !ok {
		if logger != nil {
			logger.Warn(
				fmt.Sprintf("插件清单顶层结构不是对象，已跳过：%s（来源：%s）", logpath.Display(repoRoot, infoPath), sourceRoot),
				"component", "plugins",
				"manifest_path", logpath.Display(repoRoot, infoPath),
				"source_root", sourceRoot,
			)
		}
		return plugins.Snapshot{}, false, nil
	}

	pluginID, ok := extractStringField(manifest, "id")
	if !ok {
		if logger != nil {
			logger.Warn(
				fmt.Sprintf("插件清单缺少有效 ID，已跳过：%s（来源：%s）", logpath.Display(repoRoot, infoPath), sourceRoot),
				"component", "plugins",
				"manifest_path", logpath.Display(repoRoot, infoPath),
				"source_root", sourceRoot,
			)
		}
		return plugins.Snapshot{}, false, nil
	}

	defaultConfig, defaultConfigErr := manifestDefaultConfig(manifest, filepath.Dir(infoPath))

	snapshot := plugins.Snapshot{
		PluginID:              pluginID,
		Name:                  stringField(manifest, "name"),
		Version:               stringField(manifest, "version"),
		Author:                stringField(manifest, "author"),
		License:               stringField(manifest, "license"),
		ManifestVersion:       stringField(manifest, "manifest_version"),
		PluginProtocolVersion: stringField(manifest, "plugin_protocol_version"),
		MinCoreVersion:        stringField(manifest, "min_core_version"),
		DataSchemaVersion:     stringField(manifest, "data_schema_version"),
		Concurrency:           manifestConcurrency(manifest),
		Platforms:             stringListField(manifest, "platforms"),
		Runtime:               stringField(manifest, "runtime"),
		Entry:                 stringField(manifest, "entry"),
		Description:           stringField(manifest, "description"),
		Icon:                  stringField(manifest, "icon"),
		Repo:                  stringField(manifest, "repo"),
		Homepage:              stringField(manifest, "homepage"),
		Keywords:              stringListField(manifest, "keywords"),
		Screenshots:           manifestScreenshots(manifest),
		ManagementUI:          manifestManagementUI(manifest),
		RenderTemplates:       manifestRenderTemplates(manifest),
		Help:                  manifestHelp(manifest),
		DefaultConfig:         defaultConfig,
		ManifestPath:          logpath.Display(repoRoot, infoPath),
		PackageRootPath:       filepath.Dir(infoPath),
		SourceRoot:            sourceRoot,
		SourceRoots:           []string{sourceRoot},
		RegistrationState:     plugins.RegistrationStateInstalled,
		DesiredState:          defaultDesiredStateForSourceRoot(sourceRoot),
		RuntimeState:          plugins.RuntimeStateStopped,
	}
	snapshot.DeclaredCapabilities = stringListField(manifest, "capabilities")
	snapshot.ScopeHTTPHosts = manifestCapabilityParameterList(manifest, "http_hosts")
	snapshot.ScopeStorageRoots = manifestCapabilityParameterList(manifest, "storage_roots")
	snapshot.ScopeThirdPartyAccounts = manifestCapabilityParameterList(manifest, "third_party_account_platforms")
	snapshot.ScopeWebhooks = manifestWebhookParameters(manifest)
	snapshot.ManifestCommands = manifestCommands(manifest)
	snapshot.CommandPatterns = manifestCommandPatterns(manifest)
	snapshot.DynamicCommands = manifestDynamicCommands(manifest)
	snapshot.Commands = ProjectCommands(snapshot, snapshot.DefaultConfig)

	if defaultConfigErr != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(defaultConfigErr.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	if err := validator.Validate(document); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	if err := validateCommandPatterns(snapshot.CommandPatterns); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	if err := validateManagementUIPages(snapshot.ManagementUI); err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}

	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}
	verifiedArtifact, err := artifact.Verify(snapshot.PackageRootPath, artifact.Options{ExpectedPlatform: targetPlatform})
	if err != nil {
		snapshot.Valid = false
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}
	snapshot.ArtifactVersion = verifiedArtifact.Document.ArtifactVersion
	snapshot.ArtifactTargetPlatform = verifiedArtifact.Document.TargetPlatform
	snapshot.ArtifactManifestSHA256 = verifiedArtifact.Document.ManifestSHA256
	snapshot.ArtifactFileCount = len(verifiedArtifact.Document.Files)
	snapshot.ArtifactUIAvailable = verifiedArtifact.UIAvailable
	for _, file := range verifiedArtifact.Document.Files {
		if file.Role == "backend" {
			snapshot.ArtifactBackendSHA256 = file.SHA256
			break
		}
	}

	snapshot.Valid = true
	snapshot.DisplayState = plugins.DisplayStateDiscovered
	return snapshot, true, nil
}

func trimSummary(summary string, maxLen int) string {
	singleLine := strings.Join(strings.Fields(summary), " ")
	if len(singleLine) <= maxLen {
		return singleLine
	}

	return singleLine[:maxLen-3] + "..."
}
