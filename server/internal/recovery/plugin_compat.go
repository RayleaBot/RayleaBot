package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

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

func pluginCompatibilityIssue(plugin plugins.Snapshot, targetCoreVersion, platformName string) (string, SkippedPlugin) {
	if strings.TrimSpace(plugin.MinCoreVersion) != "" && compareSemver(plugin.MinCoreVersion, targetCoreVersion) > 0 {
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
	if len(plugin.Platforms) > 0 && !contains(plugin.Platforms, platformName) {
		return "plugin.platform_mismatch", SkippedPlugin{
			PluginID:     plugin.PluginID,
			Version:      plugin.Version,
			ReasonCode:   "plugin.platform_mismatch",
			Summary:      "插件平台兼容性不满足，已保留安装目录并跳过自动启用。",
			ReviewID:     buildReviewID(plugin.PluginID, "plugin.platform_mismatch", plugin.Version),
			ReviewStatus: reviewStatusPending,
			ManualAction: "安装支持当前平台的插件包。",
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
