package app

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func (s *systemService) startupRequiredRuntimeKinds() []string {
	if s == nil {
		return nil
	}
	kinds := make([]string, 0, len(startupRuntimeKinds()))
	if strings.TrimSpace(s.state.Config.Render.BrowserPath) == "" {
		kinds = append(kinds, "chromium")
	}
	kinds = append(kinds, "python-runtime", "nodejs-runtime")
	return kinds
}

func (s *systemService) autoPrepareRuntimeEnvironments(ctx context.Context) {
	if s == nil || s.state.repoRoot == "" {
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

		inspection, err := inspectStartupRuntime(s.state.repoRoot, kind)
		if err != nil {
			issue := runtimeInspectionIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(s.state.Logger, kind, err)
			continue
		}
		if !inspection.MetadataComplete {
			issue := runtimeMetadataIssue(kind)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			continue
		}
		if inspection.PreparedStorePresent {
			s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
			continue
		}

		label := startupRuntimeLabel(kind)
		s.setStartupRuntimeState(kind, startupRuntimePending, nil)
		if s.state.Logger != nil {
			s.state.Logger.Info(
				"startup runtime prepare requested",
				"component", "app",
				"resource_kind", kind,
				"label", label,
				"cached_archive_present", inspection.CachedArchivePresent,
			)
		}

		report, err := prepareStartupRuntimeWithProgress(ctx, s.state.repoRoot, kind, func(event deps.PrepareProgress) {
			logStartupRuntimeProgress(s.state.Logger, event)
		})
		if err != nil {
			issue := startupRuntimeFailureIssue(kind, err)
			s.setStartupRuntimeState(kind, startupRuntimeFailed, &issue)
			logStartupRuntimeFailure(s.state.Logger, kind, err)
			continue
		}

		s.setStartupRuntimeState(kind, startupRuntimeReady, nil)
		if kind == "chromium" && s.renderer != nil && report.PreparedEntrypoint != "" {
			s.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
		}
		if s.state.Logger != nil {
			s.state.Logger.Info(
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
