package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/recovery"
	"rayleabot/server/internal/tasks"
)

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
}

type managedRuntimePrepareReport struct {
	Kind               string
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	PreparedEntrypoint string
}

var prepareManagedRuntimeWithReport = func(ctx context.Context, repoRoot, kind string) (*managedRuntimePrepareReport, error) {
	report, err := deps.NewManager(repoRoot).PrepareWithReport(ctx, kind)
	if err != nil {
		return nil, err
	}
	return &managedRuntimePrepareReport{
		Kind:               report.Kind,
		ArchivePath:        report.ArchivePath,
		StoreRoot:          report.StoreRoot,
		UsedPreparedStore:  report.UsedPreparedStore,
		UsedCachedArchive:  report.UsedCachedArchive,
		PreparedEntrypoint: report.PreparedEntrypoint,
	}, nil
}

func (a *App) handleSystemRecoveryRecheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		summary, err := recovery.LoadSummary(a.repoRoot)
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if summary == nil || (!summary.RequiresPostStartChecks && summary.Phase != "post_startup") {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "recovery_summary",
				"path":          recovery.SummaryPath(a.repoRoot),
			})
			return
		}

		taskID, err := a.taskExecutor.Submit("recovery.recheck", "重新检查恢复摘要", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(25, "读取恢复摘要")
			reconciled, err := a.reconcileRecoverySummary()
			if err != nil {
				return nil, err
			}
			if reconciled == nil {
				return nil, &tasks.TaskError{
					Code:    codeResourceMissing,
					Message: "恢复摘要不存在或当前不可重新检查",
					Details: map[string]any{
						"resource_type": "recovery_summary",
						"path":          recovery.SummaryPath(a.repoRoot),
					},
				}
			}
			progress.Update(90, "写入恢复摘要")
			return &tasks.ResultSummary{
				Summary: "恢复摘要已重新检查",
				Details: map[string]any{
					"recovery_summary": reconciled,
				},
			}, nil
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (a *App) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		req, err := decodeRuntimeBootstrapRequest(r)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		resources, ok := normalizeRuntimeBootstrapResources(req.Resources)
		if !ok {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		taskID, err := a.taskExecutor.Submit("runtime.bootstrap", "准备运行环境", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			results := make([]any, 0, len(resources))
			for index, kind := range resources {
				progress.Update((index*70)/len(resources), "准备 "+kind)
				report, err := prepareManagedRuntimeWithReport(ctx, a.repoRoot, kind)
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
				if kind == "chromium" && a.renderer != nil && report.PreparedEntrypoint != "" {
					a.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
				}
				results = append(results, map[string]any{
					"kind":                report.Kind,
					"archive_path":        report.ArchivePath,
					"store_root":          report.StoreRoot,
					"used_cached_archive": report.UsedCachedArchive,
					"used_prepared_store": report.UsedPreparedStore,
				})
			}

			details := map[string]any{"resources": results}
			if a.recoverySummary != nil {
				if reconciled, err := a.reconcileRecoverySummary(); err == nil && reconciled != nil {
					details["recovery_summary"] = reconciled
				}
			}

			return &tasks.ResultSummary{
				Summary: "所选资源已准备完成",
				Details: details,
			}, nil
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func decodeRuntimeBootstrapRequest(r *http.Request) (runtimeBootstrapRequest, error) {
	if r == nil || r.Body == nil {
		return runtimeBootstrapRequest{}, nil
	}
	var req runtimeBootstrapRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return runtimeBootstrapRequest{}, err
		}
		if err == io.EOF {
			return runtimeBootstrapRequest{}, nil
		}
		return runtimeBootstrapRequest{}, err
	}
	return req, nil
}

func normalizeRuntimeBootstrapResources(requested []string) ([]string, bool) {
	if len(requested) == 0 {
		return []string{"chromium", "python-runtime", "nodejs-runtime"}, true
	}
	seen := map[string]struct{}{}
	resources := make([]string, 0, len(requested))
	for _, item := range requested {
		switch item {
		case "chromium", "python-runtime", "nodejs-runtime":
		default:
			return nil, false
		}
		if _, ok := seen[item]; ok {
			return nil, false
		}
		seen[item] = struct{}{}
		resources = append(resources, item)
	}
	return resources, true
}
