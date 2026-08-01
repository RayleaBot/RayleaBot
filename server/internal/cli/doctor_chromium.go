package cli

import "github.com/RayleaBot/RayleaBot/server/internal/deps"

func chromiumMetadataIssue(manifest *deps.Manifest, platform string) DoctorIssue {
	if deps.ResourceMetadataComplete(manifest.FindResource(platform, "chromium")) {
		return DoctorIssue{
			Code:     "deps.chromium_metadata",
			Severity: "ok",
			Summary:  "图片渲染 Chromium 元数据完整。",
		}
	}
	return DoctorIssue{
		Code:        "deps.chromium_metadata_incomplete",
		Severity:    "warning",
		Summary:     "图片渲染 Chromium 元数据不完整。",
		Remediation: "请在 .deps/manifest.json 中补齐当前平台 Chromium 的来源列表、archive_format、browser 入口与 sha256。",
	}
}
