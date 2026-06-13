package app

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *systemService) submitRuntimeBootstrapTask(resources []string) (string, error) {
	if s == nil || s.taskExecutor == nil {
		return "", errSystemTaskUnavailable
	}
	return s.taskExecutor.Submit("runtime.bootstrap", "准备运行环境", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		results := make([]any, 0, len(resources))
		for index, kind := range resources {
			progress.Update((index*100)/len(resources), "正在准备 "+deps.ManagedResourceLabel(kind))
			report, err := prepareManagedRuntimeWithProgress(ctx, s.state.repoRoot, kind, func(event deps.PrepareProgress) {
				percent, summary := managedRuntimeTaskProgress(len(resources), index, event)
				progress.Update(percent, summary)
			})
			if err != nil {
				var bootstrapErr *deps.BootstrapError
				if errors.As(err, &bootstrapErr) {
					return nil, &tasks.TaskError{
						Code:    codeResourceMissing,
						Message: bootstrapErr.Message,
						Details: bootstrapErr.Details(),
					}
				}
				return nil, err
			}
			if kind == "chromium" && s.renderer != nil && report.PreparedEntrypoint != "" {
				s.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
			}
			results = append(results, map[string]any{
				"kind":                report.Kind,
				"archive_path":        report.ArchivePath,
				"store_root":          report.StoreRoot,
				"used_cached_archive": report.UsedCachedArchive,
				"used_prepared_store": report.UsedPreparedStore,
				"attempted_sources":   append([]string{}, report.AttemptedSources...),
				"selected_source":     report.SelectedSource,
			})
		}

		details := map[string]any{"resources": results}
		if s.state.recoverySummarySnapshot() != nil {
			if reconciled, err := s.reconcileRecoverySummary(); err == nil && reconciled != nil {
				details["recovery_summary"] = reconciled
			}
		}

		return &tasks.ResultSummary{
			Summary: "所选资源已准备完成",
			Details: details,
		}, nil
	})
}
