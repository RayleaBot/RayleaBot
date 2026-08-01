package system

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
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
	pluginsList := []plugins.Snapshot(nil)
	if s != nil && s.plugins != nil {
		pluginsList = s.plugins.List()
	}
	issues := s.platformDiagnostics()
	return recovery.FinalizeInput{
		Plugins:          pluginsList,
		DesiredStateRepo: s.pluginRepository,
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
	if err := recovery.SaveSummary(s.repoRootPath(), reconciled); err != nil {
		return nil, err
	}
	s.applyRecoverySummary(&reconciled)
	return &reconciled, nil
}

func (s *Service) ReconcileRecoverySummaryBestEffort(trigger string) {
	if _, err := s.reconcileRecoverySummary(); err != nil && s.currentLogger() != nil {
		s.currentLogger().Warn(
			"failed to reconcile recovery summary",
			"component", "app",
			"trigger", strings.TrimSpace(trigger),
			"err", err.Error(),
		)
	}
}

func (s *Service) applyRecoverySummary(summary *recovery.CompatibilitySummary) {
	if summary != nil && s.plugins != nil {
		for _, skipped := range summary.SkippedPlugins {
			if snapshot, ok := s.plugins.Get(skipped.PluginID); ok && snapshot.DesiredState != "disabled" {
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
			Code:        issue.Code,
			Severity:    issue.Severity,
			Summary:     issue.Summary,
			Remediation: issue.Remediation,
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
