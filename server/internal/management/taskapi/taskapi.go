package taskapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

const (
	codeInvalidRequest     = "platform.invalid_request"
	codeResourceMissing    = "platform.resource_missing"
	codeTaskNotCancellable = "platform.task_not_cancellable"
)

type Handlers struct {
	tasks           registryService
	taskExecutor    canceller
	pluginInstaller canceller
}

type registryService interface {
	List() []tasks.Snapshot
	Get(string) (tasks.Snapshot, bool)
	Update(string, tasks.Update) (tasks.Snapshot, bool)
}

type canceller interface {
	Cancel(string) bool
}

func NewHandlers(taskRegistry registryService, taskExecutor canceller, pluginInstaller canceller) *Handlers {
	return &Handlers{
		tasks:           taskRegistry,
		taskExecutor:    taskExecutor,
		pluginInstaller: pluginInstaller,
	}
}

func (h *Handlers) SetPluginInstaller(installer canceller) {
	if h == nil {
		return
	}
	h.pluginInstaller = installer
}

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
	"recovery.confirm":  {},
	"restore.apply":     {},
	"runtime.bootstrap": {},
}

type listResponse struct {
	Items []tasks.Snapshot `json:"items"`
}

type detailResponse struct {
	Task tasks.Snapshot `json:"task"`
}

type acceptedResponse struct {
	TaskID string `json:"task_id"`
}

func (h *Handlers) HandleTaskList() http.HandlerFunc {
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

		writeAuthJSON(w, http.StatusOK, listResponse{Items: items})
	}
}

func (h *Handlers) HandleTaskDetail() http.HandlerFunc {
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

		writeAuthJSON(w, http.StatusOK, detailResponse{Task: snapshot})
	}
}

func (h *Handlers) HandleTaskCancel() http.HandlerFunc {
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
			writeAuthJSON(w, http.StatusAccepted, acceptedResponse{TaskID: taskID})
			return
		}
		if h.taskExecutor != nil && h.taskExecutor.Cancel(taskID) {
			writeAuthJSON(w, http.StatusAccepted, acceptedResponse{TaskID: taskID})
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

		writeAuthJSON(w, http.StatusAccepted, acceptedResponse{TaskID: taskID})
	}
}

func writeAuthError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string) {
	writeAppError(w, r, statusCode, code, message, messageKey, nil)
}

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
