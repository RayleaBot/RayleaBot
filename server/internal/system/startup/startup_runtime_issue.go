package startup

import (
	"errors"
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func InspectionIssue(_ string, err error) recovery.CompatibilityIssue {
	var bootstrapErr *deps.BootstrapError
	if errors.As(err, &bootstrapErr) && (errors.Is(bootstrapErr.Err, os.ErrNotExist) || !strings.Contains(strings.ToLower(bootstrapErr.Err.Error()), "does not include")) {
		return recovery.CompatibilityIssue{
			Code:        "deps.manifest_missing",
			Severity:    "warning",
			Summary:     "运行环境清单缺失或无效。",
			Remediation: "请恢复有效的 .deps/manifest.json。",
		}
	}
	return recovery.CompatibilityIssue{
		Code:        "deps.manifest_platform_missing",
		Severity:    "warning",
		Summary:     "运行环境清单缺少当前平台资源。",
		Remediation: "请恢复当前平台的 .deps 资源清单。",
	}
}

func MetadataIssue(kind string) recovery.CompatibilityIssue {
	switch kind {
	case "python-runtime":
		return recovery.CompatibilityIssue{
			Code:        "deps.python_runtime_metadata_incomplete",
			Severity:    "warning",
			Summary:     "Python 运行环境元数据不完整。",
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	case "nodejs-runtime":
		return recovery.CompatibilityIssue{
			Code:        "deps.nodejs_runtime_metadata_incomplete",
			Severity:    "warning",
			Summary:     "Node.js / npm 环境元数据不完整。",
			Remediation: "请在 .deps/manifest.json 中补齐当前平台 Node.js / npm 环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	default:
		return recovery.CompatibilityIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "运行环境元数据不完整。",
			Remediation: "请补齐当前平台运行环境的 archive_format、entrypoints、来源列表与 sha256。",
		}
	}
}

func FailureIssue(kind string, err error) recovery.CompatibilityIssue {
	issue := recovery.CompatibilityIssue{
		Code:        "platform.resource_missing",
		Severity:    "warning",
		Summary:     deps.ManagedResourceLabel(kind) + "准备失败。",
		Remediation: deps.BootstrapRemediation(kind, "", ""),
	}

	var bootstrapErr *deps.BootstrapError
	if !errors.As(err, &bootstrapErr) {
		return issue
	}

	if summary := strings.TrimSpace(bootstrapErr.Message); summary != "" {
		issue.Summary = summary
		if !strings.HasSuffix(issue.Summary, "。") {
			issue.Summary += "。"
		}
	}
	if remediation := strings.TrimSpace(bootstrapErr.Remediation); remediation != "" {
		issue.Remediation = remediation
	}
	return issue
}
