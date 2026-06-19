package view

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func pluginDisplayName(snapshot plugins.Snapshot) string {
	if strings.TrimSpace(snapshot.Name) != "" {
		return snapshot.Name
	}
	return snapshot.PluginID
}

func effectivePluginRole(snapshot plugins.Snapshot) string {
	if strings.TrimSpace(snapshot.Role) != "" {
		return snapshot.Role
	}
	switch snapshot.SourceRoot {
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

func buildPluginSource(snapshot plugins.Snapshot) SourceResponse {
	root := snapshot.SourceRoot
	if root == "" && len(snapshot.SourceRoots) > 0 {
		root = snapshot.SourceRoots[0]
	}
	return SourceResponse{
		Root:              root,
		PackageSourceType: snapshot.PackageSourceType,
		PackageSourceRef:  snapshot.PackageSourceRef,
		Verified:          isVerifiedPluginSource(snapshot),
	}
}

func isVerifiedPluginSource(snapshot plugins.Snapshot) bool {
	switch snapshot.SourceRoot {
	case "plugins/builtin", "examples/plugins", "plugins/dev":
		return true
	default:
		return false
	}
}

func buildPluginTrust(role string, snapshot plugins.Snapshot) TrustResponse {
	switch role {
	case "builtin":
		return TrustResponse{Level: "official", Label: "官方"}
	case "dev":
		return TrustResponse{Level: "development", Label: "开发中"}
	case "example":
		return TrustResponse{Level: "third_party", Label: "示例"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "remote_url" {
			return TrustResponse{Level: "unverified", Label: "未验证来源"}
		}
		return TrustResponse{Level: "third_party", Label: "第三方"}
	}
}
