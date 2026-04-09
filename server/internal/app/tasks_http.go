package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

var allowedTaskStatuses = map[string]struct{}{
	"pending":     {},
	"running":     {},
	"succeeded":   {},
	"failed":      {},
	"cancelled":   {},
	"interrupted": {},
}

var allowedTaskTypes = map[string]struct{}{
	"plugin.install":    {},
	"plugin.uninstall":  {},
	"plugin.reload":     {},
	"backup.create":     {},
	"recovery.recheck":  {},
	"restore.apply":     {},
	"config.migrate":    {},
	"db.migrate":        {},
	"runtime.bootstrap": {},
	"render.preview":    {},
}

type taskListResponse struct {
	Items []tasks.Snapshot `json:"items"`
}

type taskDetailResponse struct {
	Task tasks.Snapshot `json:"task"`
}

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

func (h *taskHTTPHandlers) handleTaskList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusFilter := r.URL.Query().Get("status")
		if statusFilter != "" {
			if _, ok := allowedTaskStatuses[statusFilter]; !ok {
				writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
				return
			}
		}

		taskTypeFilter := r.URL.Query().Get("task_type")
		if taskTypeFilter != "" {
			if _, ok := allowedTaskTypes[taskTypeFilter]; !ok {
				writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
				return
			}
		}

		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 100 {
				writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
				return
			}
			limit = parsed
		}

		items := make([]tasks.Snapshot, 0)
		for _, snapshot := range h.tasks.List() {
			if statusFilter != "" && string(snapshot.Status) != statusFilter {
				continue
			}
			if taskTypeFilter != "" && snapshot.TaskType != taskTypeFilter {
				continue
			}
			items = append(items, snapshot)
		}
		if len(items) > limit {
			items = items[len(items)-limit:]
		}

		writeAuthJSON(w, http.StatusOK, taskListResponse{Items: items})
	}
}

func (h *taskHTTPHandlers) handleTaskDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "task_id")
		snapshot, ok := h.tasks.Get(taskID)
		if !ok {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "task",
				"task_id":       taskID,
			})
			return
		}

		writeAuthJSON(w, http.StatusOK, taskDetailResponse{Task: snapshot})
	}
}

func (h *taskHTTPHandlers) handleTaskCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "task_id")
		snapshot, ok := h.tasks.Get(taskID)
		if !ok {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "task",
				"task_id":       taskID,
			})
			return
		}

		if h.pluginInstaller != nil && h.pluginInstaller.Cancel(taskID) {
			writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
			return
		}
		if h.taskExecutor != nil && h.taskExecutor.Cancel(taskID) {
			writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
			return
		}

		if snapshot.Status != tasks.StatusPending {
			writeAppError(w, r, http.StatusConflict, codeTaskNotCancellable, "当前任务不可取消", "errors.platform.task_not_cancellable", map[string]any{
				"task_id": taskID,
				"status":  string(snapshot.Status),
			})
			return
		}

		cancelled := tasks.StatusCancelled
		now := time.Now().UTC()
		if _, ok := h.tasks.Update(taskID, tasks.Update{
			Status:     &cancelled,
			FinishedAt: &now,
		}); !ok {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "task",
				"task_id":       taskID,
			})
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}
