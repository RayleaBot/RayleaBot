package system

import (
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/system/startup"
)

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
	requiredKinds := startup.ManagedDiagnosticKinds()
	if len(requiredKinds) == 0 {
		return nil
	}
	issues := []recovery.CompatibilityIssue{}
	manager := deps.NewManager(s.repoRootPath())
	for _, kind := range requiredKinds {
		inspection, err := manager.Inspect(kind)
		if err != nil {
			issues = append(issues, startup.InspectionIssue(kind, err))
			continue
		}
		if !inspection.MetadataComplete {
			issues = append(issues, startup.MetadataIssue(kind))
			continue
		}
		if inspection.PreparedStorePresent {
			continue
		}
		if state, ok := s.startupRuntimeState(kind); ok {
			switch state.Phase {
			case startupRuntimePending:
				continue
			case startupRuntimeFailed:
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
