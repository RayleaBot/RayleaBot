package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const maxRecoveryConfirmNoteRunes = 500

type recoveryConfirmRequest struct {
	ReviewIDs []string `json:"review_ids"`
	Note      string   `json:"note,omitempty"`
}

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
}

type managedRuntimePrepareReport struct {
	Kind               string
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	AttemptedSources   []string
	SelectedSource     string
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
		AttemptedSources:   append([]string{}, report.AttemptedSources...),
		SelectedSource:     report.SelectedSource,
		PreparedEntrypoint: report.PreparedEntrypoint,
	}, nil
}

var prepareManagedRuntimeWithProgress = func(ctx context.Context, repoRoot, kind string, progress deps.PrepareProgressReporter) (*managedRuntimePrepareReport, error) {
	if progress == nil {
		return prepareManagedRuntimeWithReport(ctx, repoRoot, kind)
	}
	report, err := deps.NewManager(repoRoot).PrepareWithReportOptions(ctx, kind, deps.PrepareOptions{Progress: progress})
	if err != nil {
		return nil, err
	}
	return &managedRuntimePrepareReport{
		Kind:               report.Kind,
		ArchivePath:        report.ArchivePath,
		StoreRoot:          report.StoreRoot,
		UsedPreparedStore:  report.UsedPreparedStore,
		UsedCachedArchive:  report.UsedCachedArchive,
		AttemptedSources:   append([]string{}, report.AttemptedSources...),
		SelectedSource:     report.SelectedSource,
		PreparedEntrypoint: report.PreparedEntrypoint,
	}, nil
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
	case "download":
		if event.Status == "succeeded" {
			return label + "安装包已下载"
		}
		return "正在下载 " + label
	case "verify":
		return "正在校验 " + label + "安装包"
	case "extract":
		if event.Status == "succeeded" {
			return label + "已解压"
		}
		return "正在解压 " + label
	case "cleanup":
		return "正在清理未完成的 " + label + "目录"
	case "activate":
		if event.Status == "succeeded" {
			return label + "已启用"
		}
		return "正在启用 " + label
	case "complete":
		return label + "已准备完成"
	default:
		return "正在准备 " + label
	}
}

func (h *systemHTTPHandlers) handleSystemRecoveryRecheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil || h.system.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		summary, err := recovery.LoadSummary(h.system.state.repoRoot)
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if summary == nil || (!summary.RequiresPostStartChecks && summary.Phase != "post_startup") {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "recovery_summary",
				"path":          recovery.SummaryPath(h.system.state.repoRoot),
			})
			return
		}

		taskID, err := h.system.taskExecutor.Submit("recovery.recheck", "重新检查恢复摘要", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(25, "读取恢复摘要")
			reconciled, err := h.system.reconcileRecoverySummary()
			if err != nil {
				return nil, err
			}
			if reconciled == nil {
				return nil, &tasks.TaskError{
					Code:    codeResourceMissing,
					Message: "恢复摘要不存在或当前不可重新检查",
					Details: map[string]any{
						"resource_type": "recovery_summary",
						"path":          recovery.SummaryPath(h.system.state.repoRoot),
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

func (h *systemHTTPHandlers) handleSystemRecoveryConfirm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil || h.system.taskExecutor == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		req, err := decodeRecoveryConfirmRequest(r)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		reviewIDs, note, ok := normalizeRecoveryConfirmRequest(req)
		if !ok {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		summary, err := recovery.LoadSummary(h.system.state.repoRoot)
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if summary == nil || summary.Phase != "post_startup" {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "recovery_summary",
				"path":          recovery.SummaryPath(h.system.state.repoRoot),
			})
			return
		}

		if _, _, err := recovery.ConfirmSkippedPlugins(*summary, reviewIDs, "validation", note, "validation"); err != nil {
			var unknownErr *recovery.UnknownReviewIDsError
			if errors.As(err, &unknownErr) {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{
					"review_ids": unknownErr.ReviewIDs,
				})
				return
			}
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		claims, ok := ClaimsFromContext(r.Context())
		if !ok || strings.TrimSpace(claims.Subject) == "" {
			writeAuthError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		operatorID := strings.TrimSpace(claims.Subject)

		taskIDCh := make(chan string, 1)
		taskID, err := h.system.taskExecutor.Submit("recovery.confirm", "确认恢复处理结果", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			progress.Update(20, "读取恢复摘要")
			currentSummary, err := recovery.LoadSummary(h.system.state.repoRoot)
			if err != nil {
				return nil, err
			}
			if currentSummary == nil || currentSummary.Phase != "post_startup" {
				return nil, &tasks.TaskError{
					Code:    codeResourceMissing,
					Message: "恢复摘要不存在或当前不可确认",
					Details: map[string]any{
						"resource_type": "recovery_summary",
						"path":          recovery.SummaryPath(h.system.state.repoRoot),
					},
				}
			}
			progress.Update(55, "确认恢复项")
			taskID := <-taskIDCh
			confirmedSummary, confirmedReviewIDs, err := recovery.ConfirmSkippedPlugins(*currentSummary, reviewIDs, operatorID, note, taskID)
			if err != nil {
				var unknownErr *recovery.UnknownReviewIDsError
				if errors.As(err, &unknownErr) {
					return nil, &tasks.TaskError{
						Code:    codeInvalidRequest,
						Message: "请求参数不合法",
						Details: map[string]any{
							"review_ids": unknownErr.ReviewIDs,
						},
					}
				}
				return nil, err
			}
			progress.Update(85, "写入恢复摘要")
			if err := recovery.SaveSummary(h.system.state.repoRoot, confirmedSummary); err != nil {
				return nil, err
			}
			h.system.applyRecoverySummary(&confirmedSummary)

			summaryText := "所选恢复项已确认"
			if len(confirmedReviewIDs) == 0 {
				summaryText = "所选恢复项已经确认"
			}
			return &tasks.ResultSummary{
				Summary: summaryText,
				Details: map[string]any{
					"recovery_summary":     confirmedSummary,
					"confirmed_review_ids": confirmedReviewIDs,
					"operator_id":          operatorID,
					"note":                 note,
				},
			}, nil
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		taskIDCh <- taskID

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *systemHTTPHandlers) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil || h.system.taskExecutor == nil {
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

		taskID, err := h.system.taskExecutor.Submit("runtime.bootstrap", "准备运行环境", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
			results := make([]any, 0, len(resources))
			for index, kind := range resources {
				progress.Update((index*100)/len(resources), "正在准备 "+deps.ManagedResourceLabel(kind))
				report, err := prepareManagedRuntimeWithProgress(ctx, h.system.state.repoRoot, kind, func(event deps.PrepareProgress) {
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
				if kind == "chromium" && h.system.renderer != nil && report.PreparedEntrypoint != "" {
					h.system.renderer.RefreshBrowserPath(report.PreparedEntrypoint)
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
			if h.system.state.recoverySummarySnapshot() != nil {
				if reconciled, err := h.system.reconcileRecoverySummary(); err == nil && reconciled != nil {
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

func decodeRecoveryConfirmRequest(r *http.Request) (recoveryConfirmRequest, error) {
	if r == nil || r.Body == nil {
		return recoveryConfirmRequest{}, io.EOF
	}
	var req recoveryConfirmRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return recoveryConfirmRequest{}, err
		}
		return recoveryConfirmRequest{}, err
	}
	return req, nil
}

func normalizeRecoveryConfirmRequest(req recoveryConfirmRequest) ([]string, string, bool) {
	reviewIDs := make([]string, 0, len(req.ReviewIDs))
	seen := map[string]struct{}{}
	for _, reviewID := range req.ReviewIDs {
		reviewID = strings.TrimSpace(reviewID)
		if reviewID == "" {
			return nil, "", false
		}
		if _, ok := seen[reviewID]; ok {
			continue
		}
		seen[reviewID] = struct{}{}
		reviewIDs = append(reviewIDs, reviewID)
	}
	if len(reviewIDs) == 0 {
		return nil, "", false
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxRecoveryConfirmNoteRunes {
		return nil, "", false
	}
	return reviewIDs, note, true
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
