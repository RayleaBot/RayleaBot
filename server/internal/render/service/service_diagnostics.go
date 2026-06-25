package service

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func (s *Service) Diagnostics() []health.DiagnosticIssue {
	issues := make([]health.DiagnosticIssue, 0, 4)

	info, err := os.Stat(s.templatesRoot)
	switch {
	case os.IsNotExist(err):
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录缺失",
			Remediation: "请恢复仓库中的 templates 目录。",
		})
	case err != nil:
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录不可读",
			Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
		})
	case !info.IsDir():
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "模板资源目录结构无效",
			Remediation: "请恢复仓库中的 templates 目录结构。",
		})
	default:
		Seeds, err := rendertemplates.DiscoverSeeds(s.templatesRoot, s.logger)
		if err != nil {
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     "模板资源目录不可读",
				Remediation: "请确认 templates 目录存在且当前进程有读取权限。",
			})
			break
		}
		required := []string{"help.menu", "status.panel"}
		for _, templateID := range required {
			if _, ok := Seeds[templateID]; ok {
				continue
			}
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     fmt.Sprintf("渲染模板 %s 缺失", templateID),
				Remediation: "请恢复仓库中的正式模板资源。",
			})
		}
	}

	if strings.TrimSpace(s.browserPath) != "" {
		return issues
	}

	inspection, err := deps.NewDiagnostics(s.repoRoot).InspectRuntime("chromium")
	if err != nil {
		var bootstrapErr *deps.BootstrapError
		if errors.As(err, &bootstrapErr) {
			issues = append(issues, health.DiagnosticIssue{
				Code:        "platform.resource_missing",
				Severity:    "warning",
				Summary:     bootstrapErr.Message,
				Remediation: bootstrapErr.Remediation,
			})
			return issues
		}
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "图片渲染 Chromium 资源清单不可用。",
			Remediation: "请恢复 .deps/manifest.json，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if !inspection.MetadataComplete {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     deps.BootstrapSummary("chromium", inspection),
			Remediation: "请恢复当前平台图片渲染 Chromium 资源的 archive_format、entrypoints、来源列表与 sha256，或在配置中显式设置 render.browser_path。",
		})
		return issues
	}
	if inspection.PreparedStorePresent {
		return issues
	}
	if inspection.CachedArchivePresent {
		issues = append(issues, health.DiagnosticIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     "图片渲染 Chromium 已下载，但未解压。",
			Remediation: deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
		})
		return issues
	}
	issues = append(issues, health.DiagnosticIssue{
		Code:        "platform.resource_missing",
		Severity:    "warning",
		Summary:     "图片渲染 Chromium 未准备。",
		Remediation: deps.BootstrapRemediation("chromium", inspection.ArchivePath, inspection.StoreRoot),
	})
	return issues
}
