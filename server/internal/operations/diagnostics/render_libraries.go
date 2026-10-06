package diagnostics

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

var chromiumLibraries = []string{
	"libnss3.so", "libnssutil3.so", "libsmime3.so", "libnspr4.so",
	"libatk-1.0.so.0", "libatk-bridge-2.0.so.0", "libcups.so.2",
	"libdrm.so.2", "libdbus-1.so.3", "libX11.so.6", "libXcomposite.so.1",
	"libXdamage.so.1", "libXext.so.6", "libXfixes.so.3", "libXrandr.so.2",
	"libgbm.so.1", "libxcb.so.1", "libxkbcommon.so.0", "libasound.so.2",
	"libpango-1.0.so.0", "libcairo.so.2", "libglib-2.0.so.0", "libgobject-2.0.so.0",
}

func renderLibrariesIssue(cache, architecture string, err error) Issue {
	remediation := "请按安装包内 LINUX-RUNTIME.md 安装 Chromium 系统共享库，然后重新运行 doctor。"
	if err != nil || architecture == "" {
		return Issue{Code: errorcodes.DiagnosticRenderLibrariesUnavailable, Severity: "warning", Summary: "无法检查 Chromium 系统共享库。", Remediation: remediation}
	}
	available := make(map[string]bool)
	for line := range strings.SplitSeq(cache, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && strings.Contains(line, architecture) && strings.Contains(line, " => ") {
			available[fields[0]] = true
		}
	}
	var missing []string
	for _, library := range chromiumLibraries {
		if !available[library] {
			missing = append(missing, library)
		}
	}
	if len(missing) > 0 {
		return Issue{Code: errorcodes.DiagnosticRenderLibrariesMissing, Severity: "warning", Summary: fmt.Sprintf("Chromium 系统共享库缺失：%s。", strings.Join(missing, ", ")), Remediation: remediation}
	}
	return Issue{Code: errorcodes.DiagnosticRenderLibrariesOk, Severity: "ok", Summary: "Chromium 系统共享库已安装。"}
}
