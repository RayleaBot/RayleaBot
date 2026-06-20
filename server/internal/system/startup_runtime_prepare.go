package system

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/system/startup"
)

func (s *Service) startupRequiredRuntimeKinds() []string {
	if s == nil {
		return nil
	}
	kinds := make([]string, 0, len(startup.Kinds()))
	if strings.TrimSpace(s.config().Render.BrowserPath) == "" {
		kinds = append(kinds, "chromium")
	}
	kinds = append(kinds, "python-runtime", "nodejs-runtime")
	return kinds
}

func (s *Service) autoPrepareRuntimeEnvironments(ctx context.Context) {
	if s == nil || s.repoRootPath() == "" {
		return
	}

	requiredKinds := s.startupRequiredRuntimeKinds()
	s.resetStartupRuntimeStates(requiredKinds)
	if len(requiredKinds) == 0 {
		return
	}

	for _, kind := range requiredKinds {
		if err := ctx.Err(); err != nil {
			return
		}

		inspection, err := inspectStartupRuntime(s.repoRootPath(), kind)
		if err != nil {
			issue := startup.InspectionIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			startup.LogFailure(s.currentLogger(), kind, err)
			continue
		}
		if !inspection.MetadataComplete {
			issue := startup.MetadataIssue(kind)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			continue
		}
		if inspection.PreparedStorePresent {
			s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
			continue
		}

		label := startup.Label(kind)
		s.setStartupRuntimeState(kind, startupRuntimePending, nil)
		if s.currentLogger() != nil {
			s.currentLogger().Info(
				"startup runtime prepare requested",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"cached_archive_present", inspection.CachedArchivePresent,
			)
		}

		report, err := prepareStartupRuntimeWithProgress(ctx, s.repoRootPath(), kind, func(event deps.PrepareProgress) {
			startup.LogProgress(s.currentLogger(), event)
		})
		if err != nil {
			issue := startup.FailureIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			startup.LogFailure(s.currentLogger(), kind, err)
			continue
		}

		s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
		if kind == "chromium" && s.renderer != nil && report.PreparedEntrypoint != "" {
			s.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
		}
		if s.currentLogger() != nil {
			s.currentLogger().Info(
				"startup runtime prepare completed",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"used_cached_archive", report.UsedCachedArchive,
				"used_prepared_store", report.UsedPreparedStore,
				"store_root", report.StoreRoot,
			)
		}
	}

	s.ReconcileRecoverySummaryBestEffort("startup.runtime_prepare")
}

func (s *Service) AutoPrepareRuntimeEnvironments(ctx context.Context) {
	s.autoPrepareRuntimeEnvironments(ctx)
}
