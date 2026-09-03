package catalog

import (
	"fmt"
	"log/slog"
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
				fmt.Sprintf("插件清单 JSON 解析失败，已跳过：%s（来源：%s）；该插件本次不会加载，请修正清单后重新扫描。原因：%s", logpath.Display(repoRoot, infoPath), sourceRoot, err.Error()),
				"component", "plugins", "manifest_path", logpath.Display(repoRoot, infoPath), "source_root", sourceRoot, "err", err.Error(),
			)
		}
		return plugins.Snapshot{}, false, nil
	}
	raw, ok := document.(map[string]any)
	if !ok {
		if logger != nil {
			logger.Warn(
				fmt.Sprintf("插件清单顶层结构不是对象，已跳过：%s（来源：%s）；该插件本次不会加载。", logpath.Display(repoRoot, infoPath), sourceRoot),
				"component", "plugins", "manifest_path", logpath.Display(repoRoot, infoPath), "source_root", sourceRoot,
			)
		}
		return plugins.Snapshot{}, false, nil
	}
	pluginID, pluginName := manifestIdentity(raw)
	if pluginID == "" {
		if logger != nil {
			logger.Warn(
				fmt.Sprintf("插件清单缺少有效 ID，已跳过：%s（来源：%s）；该插件本次不会加载。", logpath.Display(repoRoot, infoPath), sourceRoot),
				"component", "plugins", "manifest_path", logpath.Display(repoRoot, infoPath), "source_root", sourceRoot,
			)
		}
		return plugins.Snapshot{}, false, nil
	}
	invalidSnapshot := plugins.Snapshot{
		PluginID: pluginID, Name: pluginName, ManifestPath: logpath.Display(repoRoot, infoPath),
		SourceRoot: sourceRoot, SourceRoots: []string{sourceRoot},
		RegistrationState: plugins.RegistrationStateInstalled,
		DesiredState:      plugins.DesiredStateDisabled, RuntimeState: plugins.RuntimeStateStopped,
		DisplayState: plugins.DisplayStateInvalidManifest,
	}
	if err := validator.Validate(document); err != nil {
		invalidSnapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return invalidSnapshot, true, nil
	}
	manifest, err := decodeManifest(document)
	if err != nil {
		invalidSnapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return invalidSnapshot, true, nil
	}
	if err := validateManifestSemantics(manifest); err != nil {
		invalidSnapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return invalidSnapshot, true, nil
	}
	snapshot, err := projectManifest(manifest, infoPath, sourceRoot, repoRoot)
	if err != nil {
		invalidSnapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return invalidSnapshot, true, nil
	}
	snapshot.ManifestPath = logpath.Display(repoRoot, infoPath)

	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}
	verifiedArtifact, err := artifact.Verify(snapshot.PackageRootPath, artifact.Options{ExpectedPlatform: targetPlatform})
	if err != nil {
		snapshot.DisplayState = plugins.DisplayStateInvalidManifest
		snapshot.ValidationSummary = trimSummary(err.Error(), maxSummaryChars)
		return snapshot, true, nil
	}
	snapshot.ArtifactVersion = verifiedArtifact.Document.ArtifactVersion
	snapshot.ArtifactTargetPlatform = verifiedArtifact.Document.TargetPlatform
	snapshot.ArtifactUIAvailable = verifiedArtifact.UIAvailable
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
