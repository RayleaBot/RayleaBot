package system

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const codeResourceMissing = "platform.resource_missing"

var errSystemTaskUnavailable = errors.New("system task service unavailable")

var prepareManagedRuntimeWithReport = func(ctx context.Context, repoRoot, kind string) (*managedRuntimePrepareReport, error) {
	report, err := deps.NewRuntime(repoRoot).PrepareWithReport(ctx, kind)
	if err != nil {
		return nil, err
	}
	return runtimePrepareReportFromDeps(report), nil
}

var prepareManagedRuntimeWithProgress = func(ctx context.Context, repoRoot, kind string, progress deps.PrepareProgressReporter) (*managedRuntimePrepareReport, error) {
	if progress == nil {
		return prepareManagedRuntimeWithReport(ctx, repoRoot, kind)
	}
	report, err := deps.NewRuntime(repoRoot).PrepareWithReportOptions(ctx, kind, deps.PrepareOptions{Progress: progress})
	if err != nil {
		return nil, err
	}
	return runtimePrepareReportFromDeps(report), nil
}

type managedRuntimePrepareReport struct {
	Kind               string
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	UsedSystemBrowser  bool
	AttemptedSources   []string
	SelectedSource     string
	PreparedEntrypoint string
}

func runtimePrepareReportFromDeps(report *deps.PrepareReport) *managedRuntimePrepareReport {
	if report == nil {
		return nil
	}
	return &managedRuntimePrepareReport{
		Kind:               report.Kind,
		ArchivePath:        report.ArchivePath,
		StoreRoot:          report.StoreRoot,
		UsedPreparedStore:  report.UsedPreparedStore,
		UsedCachedArchive:  report.UsedCachedArchive,
		UsedSystemBrowser:  report.UsedSystemBrowser,
		AttemptedSources:   append([]string{}, report.AttemptedSources...),
		SelectedSource:     report.SelectedSource,
		PreparedEntrypoint: report.PreparedEntrypoint,
	}
}

func (s *Service) SubmitRuntimeBootstrapTask(resources []string) (string, error) {
	if s.taskExecutor == nil {
		return "", errSystemTaskUnavailable
	}
	return s.taskExecutor.Submit("runtime.bootstrap", "准备运行环境", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		results := make([]any, 0, len(resources))
		for index, kind := range resources {
			progress.Update((index*100)/len(resources), "正在准备 "+deps.ManagedResourceLabel(kind))
			report, err := prepareManagedRuntimeWithProgress(ctx, s.repoRootPath(), kind, func(event deps.PrepareProgress) {
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
		if s.recoverySummarySnapshot() != nil {
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

func managedRuntimeTaskProgress(total, index int, event deps.PrepareProgress) (int, string) {
	if total <= 0 {
		total = 1
	}
	base := (index * 100) / total
	share := 100 / total
	stageProgress := event.Progress
	if stageProgress < 0 {
		stageProgress = 0
	}
	if stageProgress > 100 {
		stageProgress = 100
	}
	percent := base + (share*stageProgress)/100
	if percent > 99 && event.Status != "succeeded" {
		percent = 99
	}
	if percent > 100 {
		percent = 100
	}
	summary := strings.TrimSpace(event.Summary)
	if summary == "" {
		summary = runtimePrepareStageSummary(event)
	}
	return percent, summary
}

func runtimePrepareStageSummary(event deps.PrepareProgress) string {
	label := strings.TrimSpace(event.Label)
	if label == "" {
		label = deps.ManagedResourceLabel(event.Kind)
	}
	switch event.Stage {
	case "probe":
		return "正在测试 " + deps.ManagedResourceText(event.Kind, "下载来源")
	case "download":
		if event.Status == "succeeded" {
			return deps.ManagedResourceText(event.Kind, "安装包已下载")
		}
		return "正在下载 " + label
	case "verify":
		return "正在校验 " + deps.ManagedResourceText(event.Kind, "安装包")
	case "extract":
		if event.Status == "succeeded" {
			return deps.ManagedResourceText(event.Kind, "已解压")
		}
		return "正在解压 " + label
	case "cleanup":
		return "正在清理未完成的 " + label + "目录"
	case "activate":
		if event.Status == "succeeded" {
			return deps.ManagedResourceText(event.Kind, "已启用")
		}
		return "正在启用 " + label
	case "complete":
		return deps.ManagedResourceText(event.Kind, "已准备完成")
	default:
		return "正在准备 " + label
	}
}
