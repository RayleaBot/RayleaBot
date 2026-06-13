package cli

import (
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func runtimeMetadataIssue(
	manifest *depsManifest,
	platform string,
	kind string,
	label string,
	okCode string,
	incompleteCode string,
) DoctorIssue {
	resource := manifest.findResource(platform, kind)
	if manifestResourceMetadataComplete(resource) {
		return DoctorIssue{
			Code:     okCode,
			Severity: "ok",
			Summary:  label + "元数据完整。",
		}
	}
	return DoctorIssue{
		Code:        incompleteCode,
		Severity:    "warning",
		Summary:     label + "元数据不完整。",
		Remediation: "请在 .deps/manifest.json 中补齐当前平台 " + label + " 的来源列表、archive_format、entrypoints 与 sha256。",
	}
}

func managedRuntimeDoctorIssues(repoRoot string) []DoctorIssue {
	return []DoctorIssue{
		managedRuntimeDoctorIssue(
			repoRoot,
			"python-runtime",
			"runtime.python_managed_ready",
			"Python 运行环境已准备完成。",
			"Python 运行环境已下载，启动时会解压。",
			"Python 运行环境已纳入启动流程。",
			"Python 运行环境当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Python 运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		),
		managedRuntimeDoctorIssue(
			repoRoot,
			"nodejs-runtime",
			"runtime.node_managed_ready",
			"Node.js / npm 环境已准备完成。",
			"Node.js / npm 环境已下载，启动时会解压。",
			"Node.js / npm 环境已纳入启动流程。",
			"Node.js / npm 环境当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		),
		managedRuntimeDoctorIssue(
			repoRoot,
			"nodejs-runtime",
			"runtime.npm_managed_ready",
			"npm 已准备完成。",
			"npm 已下载，启动时会解压。",
			"npm 已纳入启动流程。",
			"npm 当前不可准备。",
			"请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		),
	}
}

func managedRuntimeDoctorIssue(
	repoRoot string,
	kind string,
	code string,
	readySummary string,
	cachedSummary string,
	onDemandSummary string,
	warningSummary string,
	metadataRemediation string,
) DoctorIssue {
	inspection, err := deps.NewManager(repoRoot).Inspect(kind)
	if err != nil {
		var bootstrapErr *deps.BootstrapError
		if errors.As(err, &bootstrapErr) {
			return DoctorIssue{
				Code:        code,
				Severity:    "warning",
				Summary:     warningSummary,
				Remediation: bootstrapErr.Remediation,
			}
		}
		return DoctorIssue{
			Code:        code,
			Severity:    "warning",
			Summary:     warningSummary,
			Remediation: metadataRemediation,
		}
	}
	if !inspection.MetadataComplete {
		return DoctorIssue{
			Code:        code,
			Severity:    "warning",
			Summary:     warningSummary,
			Remediation: metadataRemediation,
		}
	}
	switch {
	case inspection.PreparedStorePresent:
		return DoctorIssue{Code: code, Severity: "ok", Summary: readySummary}
	case inspection.CachedArchivePresent:
		return DoctorIssue{Code: code, Severity: "ok", Summary: cachedSummary}
	default:
		return DoctorIssue{Code: code, Severity: "ok", Summary: onDemandSummary}
	}
}
