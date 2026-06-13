package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) renderFooter(plugin *PluginContext) string {
	pluginName := systemTemplatePlugin
	pluginVersion := developmentVersion
	if plugin != nil {
		if name := strings.TrimSpace(plugin.Name); name != "" {
			pluginName = name
		}
		if version := displayVersion(plugin.Version); version != "" {
			pluginVersion = version
		}
	}

	replacer := strings.NewReplacer(
		"{{rayleabot_version}}", displayVersion(detectRenderCoreVersion(s.repoRoot)),
		"{{plugin_name}}", pluginName,
		"{{plugin_version}}", pluginVersion,
	)
	return replacer.Replace(s.currentFooterTemplate())
}

func displayVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "0.0.0-dev" {
		return developmentVersion
	}
	return version
}

func detectRenderCoreVersion(repoRoot string) string {
	content, err := os.ReadFile(filepath.Join(repoRoot, "build_info.json"))
	if err != nil {
		return developmentVersion
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return developmentVersion
	}
	return displayVersion(payload.Version)
}
