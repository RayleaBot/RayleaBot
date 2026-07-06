package system

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func (s *Service) RefreshRecoverySummary() {
	if s == nil || s.repoRootPath() == "" {
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

func (s *Service) reconcileRecoverySummary() (*recovery.CompatibilitySummary, error) {
	if s == nil || s.repoRootPath() == "" {
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
	if s == nil {
		return
	}
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
	s.setRecoverySummary(summary)
	s.PublishStatusSnapshot()
}

func (s *Service) renderDiagnostics() []recovery.CompatibilityIssue {
	if s == nil || s.renderer == nil {
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

func (s *Service) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	if s == nil || s.repoRootPath() == "" {
		return nil
	}
	requiredKinds := managedDiagnosticKinds()
	if len(requiredKinds) == 0 {
		return nil
	}
	issues := []recovery.CompatibilityIssue{}
	diagnostics := deps.NewDiagnostics(s.repoRootPath())
	for _, kind := range requiredKinds {
		inspection, err := diagnostics.InspectRuntime(kind)
		if err != nil {
			issues = append(issues, startupInspectionIssue(kind, err))
			continue
		}
		if !inspection.MetadataComplete {
			issues = append(issues, startupMetadataIssue(kind))
			continue
		}
		if inspection.PreparedStorePresent {
			continue
		}
		if state, ok := s.startupRuntimeState(kind); ok {
			switch state.Phase {
			case StartupRuntimePhasePending:
				continue
			case StartupRuntimePhaseFailed:
				if state.Issue != nil {
					issues = append(issues, *state.Issue)
					continue
				}
			}
		}
		label := deps.ManagedResourceLabel(kind)
		summary := label + "尚未准备完成。"
		if inspection.CachedArchivePresent {
			summary = label + "已下载，但未解压。"
		}
		issues = append(issues, recovery.CompatibilityIssue{
			Code:        "platform.resource_missing",
			Severity:    "warning",
			Summary:     summary,
			Remediation: deps.BootstrapRemediation(kind, inspection.ArchivePath, inspection.StoreRoot),
		})
	}
	return issues
}

func (s *Service) ManagedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return s.managedRuntimeDiagnostics(pluginsList)
}

func (s *Service) platformDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	items := s.renderDiagnostics()
	items = append(items, s.managedRuntimeDiagnostics(pluginsList)...)
	if len(items) == 0 {
		return nil
	}
	return items
}
