package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/operations/recovery"
)

func (s *Service) RefreshRecoverySummary() {
	if s.repoRootPath() == "" {
		return
	}

	summary, err := recovery.LoadSummary(s.repoRootPath())
	if err != nil || summary == nil {
		s.applyRecoverySummary(summary)
		return
	}
	if summary.RequiresPostStartChecks || recovery.NeedsSummaryNormalization(*summary) {
		reconciled, reconcileErr := s.reconcileRecoverySummary()
		if reconcileErr == nil && reconciled != nil {
			summary = reconciled
		}
	}
	s.applyRecoverySummary(summary)
}

func (s *Service) recoveryFinalizeInput() recovery.FinalizeInput {
	issues := s.platformDiagnostics()
	return recovery.FinalizeInput{
		Plugins: s.plugins.List(),
		Readiness: recovery.RuntimeReadiness{
			RuntimeReady:  len(issues) == 0,
			RuntimeIssues: issues,
		},
	}
}

func (s *Service) reconcileRecoverySummary() (*recovery.CompatibilitySummary, error) {
	if s.repoRootPath() == "" {
		return nil, nil
	}
	summary, err := recovery.LoadSummary(s.repoRootPath())
	if err != nil || summary == nil {
		return summary, err
	}
	if !summary.RequiresPostStartChecks && summary.Phase != "post_startup" {
		return nil, nil
	}

	reconciled := recovery.Finalize(*summary, s.recoveryFinalizeInput())
	if err := s.persistSkippedPluginsDisabled(reconciled.SkippedPlugins); err != nil {
		return nil, err
	}
	if err := recovery.SaveSummary(s.repoRootPath(), reconciled); err != nil {
		return nil, err
	}
	s.applyRecoverySummary(&reconciled)
	return &reconciled, nil
}

// persistSkippedPluginsDisabled stores the disabled state before the summary is
// saved. A failed write leaves the saved summary unchanged, so the next
// reconciliation retries it instead of letting the skipped plugin start again.
func (s *Service) persistSkippedPluginsDisabled(skipped []recovery.SkippedPlugin) error {
	for _, plugin := range skipped {
		snapshot, ok := s.plugins.Get(plugin.PluginID)
		if !ok || snapshot.DesiredState == "disabled" {
			continue
		}
		if err := s.pluginRepository.SaveDesiredState(context.Background(), plugin.PluginID, "disabled", time.Now().UTC()); err != nil {
			return fmt.Errorf("disable skipped plugin %s: %w", plugin.PluginID, err)
		}
	}
	return nil
}

func (s *Service) ReconcileRecoverySummaryBestEffort(trigger string) {
	if _, err := s.reconcileRecoverySummary(); err != nil && s.currentLogger() != nil {
		s.currentLogger().Warn(
			"恢复检查结果更新失败，首页继续显示上次结果",
			"component", "app",
			"trigger", strings.TrimSpace(trigger),
			"err", err.Error(),
		)
	}
}

func (s *Service) applyRecoverySummary(summary *recovery.CompatibilitySummary) {
	if summary != nil {
		for _, skipped := range summary.SkippedPlugins {
			if snapshot, ok := s.plugins.Get(skipped.PluginID); ok && snapshot.DesiredState != "disabled" {
				// SetDesiredState only fails when the plugin was removed or already
				// disabled after the lookup, which leaves nothing to project.
				_, _ = s.plugins.SetDesiredState(skipped.PluginID, "disabled")
			}
		}
	}
	s.setRecoverySummary(summary)
	s.PublishStatusSnapshot()
}

func (s *Service) renderDiagnostics() []recovery.CompatibilityIssue {
	if s.renderer == nil {
		return nil
	}
	diagnostics := s.renderer.Diagnostics()
	if len(diagnostics) == 0 {
		return nil
	}
	items := make([]recovery.CompatibilityIssue, 0, len(diagnostics))
	for _, issue := range diagnostics {
		items = append(items, recovery.CompatibilityIssue{
			Code:             issue.Code,
			Severity:         issue.Severity,
			Summary:          issue.Summary,
			Remediation:      issue.Remediation,
			RuntimeResources: append([]string(nil), issue.RuntimeResources...),
		})
	}
	return items
}

func (s *Service) platformDiagnostics() []recovery.CompatibilityIssue {
	items := s.renderDiagnostics()
	if len(items) == 0 {
		return nil
	}
	return items
}
