package plugins

import "strings"

func buildSourceView(snapshot Snapshot) SourceView {
	root := snapshot.SourceRoot
	if root == "" && len(snapshot.SourceRoots) > 0 {
		root = snapshot.SourceRoots[0]
	}
	return SourceView{
		Root:              root,
		PackageSourceType: snapshot.PackageSourceType,
		PackageSourceRef:  snapshot.PackageSourceRef,
		Verified:          isVerifiedSourceView(snapshot),
	}
}

func isVerifiedSourceView(snapshot Snapshot) bool {
	switch snapshot.SourceRoot {
	case "plugins/builtin", "examples/plugins", "plugins/dev":
		return true
	default:
		return false
	}
}

func buildTrustView(role string, snapshot Snapshot) TrustView {
	switch role {
	case "builtin":
		return TrustView{Level: "official", Label: "官方"}
	case "dev":
		return TrustView{Level: "development", Label: "开发中"}
	case "example":
		return TrustView{Level: "third_party", Label: "示例"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "remote_url" {
			return TrustView{Level: "unverified", Label: "未验证来源"}
		}
		return TrustView{Level: "third_party", Label: "第三方"}
	}
}

func summaryViewDisplayName(snapshot Snapshot) string {
	if strings.TrimSpace(snapshot.Name) != "" {
		return snapshot.Name
	}
	return snapshot.PluginID
}

func summaryViewRole(snapshot Snapshot) string {
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
