package diagnostics

import "github.com/RayleaBot/RayleaBot/server/internal/platform/deps"

func managedRuntimeMetadataIssue(manifest *deps.Manifest, platform, kind string) Issue {
	label := deps.ManagedResourceLabel(kind)
	if deps.ResourceMetadataComplete(manifest.FindResource(platform, kind)) {
		return Issue{
			Code:     "deps." + kind + "_metadata",
			Severity: "ok",
			Summary:  label + " 元数据完整。",
		}
	}
	return Issue{
		Code:        "deps." + kind + "_metadata_incomplete",
		Severity:    "warning",
		Summary:     label + " 元数据不完整。",
		Remediation: "请在 .deps/manifest.json 中补齐当前平台" + label + "的来源列表、archive_format、入口文件与 sha256。",
	}
}
