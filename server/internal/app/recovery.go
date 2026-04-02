package app

import (
	"time"

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
				RuntimeReady:  len(a.renderDiagnostics()) == 0,
				RuntimeIssues: a.renderDiagnostics(),
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
