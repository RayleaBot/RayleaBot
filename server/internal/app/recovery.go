package app

import (
	"time"

	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/recovery"
)

func (a *App) refreshRecoverySummary() {
	if a == nil || a.repoRoot == "" {
		return
	}

	summary, err := recovery.LoadSummary(a.repoRoot)
	if err != nil || summary == nil {
		a.recoverySummary = summary
		return
	}
	if summary.RequiresPostStartChecks {
		finalized := recovery.Finalize(*summary, recovery.FinalizeInput{
			Plugins:          a.Plugins.List(),
			DesiredStateRepo: a.pluginRepository,
			Readiness: recovery.RuntimeReadiness{
				RuntimeReady:  len(a.platformDiagnostics()) == 0,
				RuntimeIssues: a.platformDiagnostics(),
			},
		})
		if err := recovery.SaveSummary(a.repoRoot, finalized); err == nil {
			summary = &finalized
		}
	}
	if summary != nil {
		for _, skipped := range summary.SkippedPlugins {
			if snapshot, ok := a.Plugins.Get(skipped.PluginID); ok && snapshot.DesiredState != "disabled" {
				_, _ = a.Plugins.SetDesiredState(skipped.PluginID, "disabled")
			}
		}
	}
	a.recoverySummary = summary
}

func (a *App) renderDiagnostics() []recovery.CompatibilityIssue {
	if a == nil || a.renderer == nil {
		return nil
	}
	diagnostics := a.renderer.Diagnostics()
	if len(diagnostics) == 0 {
		return nil
	}
	items := make([]recovery.CompatibilityIssue, 0, len(diagnostics))
	for _, issue := range diagnostics {
		items = append(items, recovery.CompatibilityIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return items
}

func (a *App) managedRuntimeDiagnostics() []recovery.CompatibilityIssue {
	if a == nil || a.repoRoot == "" {
		return nil
	}
	manifest, err := deps.LoadManifest(a.repoRoot)
	if err != nil {
		return []recovery.CompatibilityIssue{{
			Code:        "deps.manifest_missing",
			Severity:    "warning",
			Summary:     "受控运行时清单缺失或无效。",
			Remediation: "请恢复有效的 .deps/manifest.json。",
		}}
	}
	currentPlatform := deps.CurrentPlatform()
	if !manifest.HasPlatform(currentPlatform) {
		return []recovery.CompatibilityIssue{{
			Code:        "deps.manifest_platform_missing",
			Severity:    "warning",
			Summary:     "受控运行时清单缺少当前平台资源。",
			Remediation: "请恢复当前平台的 .deps 资源清单。",
		}}
	}

	issues := []recovery.CompatibilityIssue{}
	for _, item := range []struct {
		kind        string
		code        string
		summary     string
		remediation string
	}{
		{
			kind:        "python-runtime",
			code:        "deps.python_runtime_metadata_incomplete",
			summary:     "受控 Python 运行时元数据不完整。",
			remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行时的 archive_format、entrypoints、source 与 sha256。",
		},
		{
			kind:        "nodejs-runtime",
			code:        "deps.nodejs_runtime_metadata_incomplete",
			summary:     "受控 Node.js 运行时元数据不完整。",
			remediation: "请在 .deps/manifest.json 中补齐当前平台 Node.js 运行时的 archive_format、entrypoints、source 与 sha256。",
		},
	} {
		if deps.ResourceMetadataComplete(manifest.FindResource(currentPlatform, item.kind)) {
			continue
		}
		issues = append(issues, recovery.CompatibilityIssue{
			Code:        item.code,
			Severity:    "warning",
			Summary:     item.summary,
			Remediation: item.remediation,
		})
	}
	return issues
}

func (a *App) platformDiagnostics() []recovery.CompatibilityIssue {
	items := a.renderDiagnostics()
	items = append(items, a.managedRuntimeDiagnostics()...)
	if len(items) == 0 {
		return nil
	}
	return items
}

func (a *App) recoverySummarySnapshot() *recovery.CompatibilitySummary {
	if a == nil || a.recoverySummary == nil {
		return nil
	}
	summary := *a.recoverySummary
	if summary.UpdatedAt == "" {
		summary.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return &summary
}

func recoveryIssuesToHealth(issues []recovery.CompatibilityIssue) []health.DiagnosticIssue {
	if len(issues) == 0 {
		return nil
	}
	items := make([]health.DiagnosticIssue, 0, len(issues))
	for _, issue := range issues {
		items = append(items, health.DiagnosticIssue{
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
		})
	}
	return items
}
