package diagnostics

import "github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"

const longPathsRemediation = "请启用组策略“计算机配置 > 管理模板 > 系统 > 文件系统 > 启用 Win32 长路径”，或将 HKLM\\SYSTEM\\CurrentControlSet\\Control\\FileSystem\\LongPathsEnabled 设为 DWORD 1；注册表值会被进程缓存，修改后可能需要重启 Windows。"

func longPathsIssue(value uint64, readErr error) Issue {
	if readErr != nil {
		return Issue{
			Code:        errorcodes.DiagnosticWindowsLongPathsUnavailable,
			Severity:    "warning",
			Summary:     "无法读取 Windows 长路径支持状态。",
			Remediation: longPathsRemediation,
		}
	}
	if value == 1 {
		return Issue{
			Code:     errorcodes.DiagnosticWindowsLongPathsEnabled,
			Severity: "ok",
			Summary:  "Windows 长路径支持已启用。",
		}
	}
	return Issue{
		Code:        errorcodes.DiagnosticWindowsLongPathsDisabled,
		Severity:    "warning",
		Summary:     "Windows 长路径支持未启用。",
		Remediation: longPathsRemediation,
	}
}
