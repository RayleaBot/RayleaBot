package app

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func (s *systemService) RefreshRecoverySummary() {
	if s == nil || s.state.repoRoot == "" {
		return
	}

	summary, err := recovery.LoadSummary(s.state.repoRoot)
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

func (s *systemService) recoveryFinalizeInput() recovery.FinalizeInput {
	pluginsList := []plugins.Snapshot(nil)
	if s != nil && s.plugins != nil {
		pluginsList = s.plugins.List()
	}
	issues := s.platformDiagnostics(pluginsList)
	return recovery.FinalizeInput{
		Plugins:          pluginsList,
		DesiredStateRepo: s.pluginRepository,
		Readiness: recovery.RuntimeReadiness{
			RuntimeReady:  len(issues) == 0,
			RuntimeIssues: issues,
		},
	}
}

func (s *systemService) reconcileRecoverySummary() (*recovery.CompatibilitySummary, error) {
	if s == nil || s.state.repoRoot == "" {
		return nil, nil
	}
	summary, err := recovery.LoadSummary(s.state.repoRoot)
	if err != nil || summary == nil {
		return summary, err
	}
	if !summary.RequiresPostStartChecks && summary.Phase != "post_startup" {
		return nil, nil
	}

	reconciled := recovery.Finalize(*summary, s.recoveryFinalizeInput())
	if err := recovery.SaveSummary(s.state.repoRoot, reconciled); err != nil {
		return nil, err
	}
	s.applyRecoverySummary(&reconciled)
	return &reconciled, nil
}

func (s *systemService) ReconcileRecoverySummaryBestEffort(trigger string) {
	if s == nil {
		return
	}
	if _, err := s.reconcileRecoverySummary(); err != nil && s.state.Logger != nil {
		s.state.Logger.Warn(
			"failed to reconcile recovery summary",
			"component", "app",
			"trigger", strings.TrimSpace(trigger),
			"err", err.Error(),
		)
	}
}

func (s *systemService) applyRecoverySummary(summary *recovery.CompatibilitySummary) {
	if s == nil {
		return
	}
	if summary != nil && s.plugins != nil {
		for _, skipped := range summary.SkippedPlugins {
			if snapshot, ok := s.plugins.Get(skipped.PluginID); ok && snapshot.DesiredState != "disabled" {
				_, _ = s.plugins.SetDesiredState(skipped.PluginID, "disabled")
			}
		}
	}
	s.state.setRecoverySummary(summary)
	s.publishStatusSnapshot()
}
