package management

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type SystemRoutes struct {
	Handlers *SystemHandlers
	Metrics  http.Handler
}

type SystemHandlers struct {
	system    CoreService
	scheduler SchedulerService
}

type CoreService interface {
	GetTaskStatus(string) (systemsvc.TaskStatus, bool)
	CurrentReadiness() health.ReadinessReport
	DiagnosticsSnapshot(context.Context) systemsvc.DiagnosticsSnapshot
	BuildDiagnosticsArchive(context.Context) ([]byte, error)
	SubmitSystemBackupTask() (string, error)
	ValidateRecoveryConfirmRequest([]string, string) *systemsvc.Error
	SubmitRecoveryRecheckTask() (string, *systemsvc.Error)
	SubmitRecoveryConfirmTask([]string, string, string) (string, *systemsvc.Error)
	SubmitRuntimeBootstrapTask([]string) (string, error)
}

type SchedulerMetadataService interface {
	SchedulerPluginName(string) string
	SchedulerTimezone() string
}

type SchedulerService interface {
	ListJobs() (schedulerJobListResponse, bool)
	TriggerJob(context.Context, string) (schedulerJobTriggerResponse, *SystemHTTPError)
}

type SchedulerEngineService interface {
	Jobs() []scheduler.Job
	Trigger(context.Context, string) (scheduler.Job, error)
}

const (
	systemCodePermissionDenied = errorcodes.PermissionDenied
	systemCodeInvalidRequest   = errorcodes.PlatformInvalidRequest
	systemCodeResourceMissing  = errorcodes.PlatformResourceNotFound
	systemCodeInternalError    = errorcodes.PlatformInternalError
	systemCodeTaskQueueFull    = errorcodes.PlatformTaskQueueFull
)

type SystemHTTPError struct {
	code    string
	details map[string]any
}

func InternalSystemHTTPError() *SystemHTTPError {
	return &SystemHTTPError{
		code: systemCodeInternalError,
	}
}

func InvalidSystemHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		code:    systemCodeInvalidRequest,
		details: details,
	}
}

func MissingSystemResourceHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		code:    systemCodeResourceMissing,
		details: details,
	}
}

func TaskQueueFullSystemHTTPError() *SystemHTTPError {
	return &SystemHTTPError{
		code: systemCodeTaskQueueFull,
	}
}

func WriteSystemHTTPError(w http.ResponseWriter, r *http.Request, err *SystemHTTPError) {
	if err == nil {
		return
	}
	httpapi.WriteError(w, r, err.code, err.details)
}

func WriteSystemError(w http.ResponseWriter, r *http.Request, err *systemsvc.Error) {
	WriteSystemHTTPError(w, r, systemHTTPErrorFromError(err))
}

func systemHTTPErrorFromError(err *systemsvc.Error) *SystemHTTPError {
	if err == nil {
		return nil
	}
	switch err.Reason {
	case systemsvc.ErrorReasonInvalidRequest:
		return InvalidSystemHTTPError(err.Details)
	case systemsvc.ErrorReasonResourceMissing:
		return MissingSystemResourceHTTPError(err.Details)
	case systemsvc.ErrorReasonTaskQueueFull:
		return TaskQueueFullSystemHTTPError()
	default:
		return InternalSystemHTTPError()
	}
}

type taskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

func NewSystemHandlers(system CoreService, schedulerEngine ...SchedulerEngineService) *SystemHandlers {
	var schedulerValue SchedulerService
	if len(schedulerEngine) > 0 {
		metadata, _ := system.(SchedulerMetadataService)
		schedulerValue = newSchedulerHTTPService(metadata, schedulerEngine[0])
	}
	return &SystemHandlers{system: system, scheduler: schedulerValue}
}

func NewSchedulerHandlers(metadata SchedulerMetadataService, schedulerEngine SchedulerEngineService) *SystemHandlers {
	return &SystemHandlers{scheduler: newSchedulerHTTPService(metadata, schedulerEngine)}
}

func NewSystemRoutes(handlers *SystemHandlers, metrics http.Handler) SystemRoutes {
	return SystemRoutes{Handlers: handlers, Metrics: metrics}
}

func (h *SystemHandlers) CurrentReadiness() health.ReadinessReport {
	if h.system == nil {
		return health.ReadinessReport{Status: "failed", Reason: "system service unavailable"}
	}
	return h.system.CurrentReadiness()
}

func (h *SystemHandlers) HandleSystemBackup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}

		taskID, err := h.system.SubmitSystemBackupTask()
		if err != nil {
			if errors.Is(err, tasks.ErrQueueFull) {
				WriteSystemHTTPError(w, r, TaskQueueFullSystemHTTPError())
				return
			}
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *SystemHandlers) HandleSystemDiagnostics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.system.DiagnosticsSnapshot(r.Context()))
	}
}

func (h *SystemHandlers) HandleSystemDiagnosticsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}
		archive, err := h.system.BuildDiagnosticsArchive(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="rayleabot-diagnostics.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}
}

func (h *SystemHandlers) HandleSystemRuntimeBootstrap() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}

		req, err := decodeRuntimeBootstrapRequest(w, r)
		if err != nil {
			httpapi.WriteError(w, r, systemCodeInvalidRequest, nil)
			return
		}

		resources, ok := normalizeRuntimeBootstrapResources(req.Resources)
		if !ok {
			httpapi.WriteError(w, r, systemCodeInvalidRequest, nil)
			return
		}

		taskID, err := h.system.SubmitRuntimeBootstrapTask(resources)
		if err != nil {
			if errors.Is(err, tasks.ErrQueueFull) {
				WriteSystemHTTPError(w, r, TaskQueueFullSystemHTTPError())
				return
			}
			httpapi.WriteError(w, r, systemCodeInternalError, nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
}

func decodeRuntimeBootstrapRequest(w http.ResponseWriter, r *http.Request) (runtimeBootstrapRequest, error) {
	var req runtimeBootstrapRequest
	if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil && !errors.Is(err, io.EOF) {
		return runtimeBootstrapRequest{}, err
	}
	return req, nil
}

func normalizeRuntimeBootstrapResources(requested []string) ([]string, bool) {
	if len(requested) == 0 {
		return []string{"chromium", "ffmpeg"}, true
	}
	seen := map[string]struct{}{}
	resources := make([]string, 0, len(requested))
	for _, item := range requested {
		switch item {
		case "chromium", "ffmpeg":
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

func (routes SystemRoutes) RegisterProtectedRoutes(router chi.Router) {
	registerSystemProtectedRoutes(router, routes.Handlers, routes.Metrics)
}

func (h *SystemHandlers) RegisterProtectedRoutes(router chi.Router, metricsHandler http.Handler) {
	registerSystemProtectedRoutes(router, h, metricsHandler)
}

func registerSystemProtectedRoutes(router chi.Router, h *SystemHandlers, metricsHandler http.Handler) {
	router.Get("/api/system/tasks/{task_id}", h.HandleTaskStatus())
	router.Post("/api/system/backup", h.HandleSystemBackup())
	router.Post("/api/system/recovery/recheck", h.HandleSystemRecoveryRecheck())
	router.Post("/api/system/recovery/confirm", h.HandleSystemRecoveryConfirm())
	router.Post("/api/system/runtime/bootstrap", h.HandleSystemRuntimeBootstrap())
	router.Get("/api/system/diagnostics", h.HandleSystemDiagnostics())
	router.Get("/api/system/diagnostics/export", h.HandleSystemDiagnosticsExport())
	if metricsHandler != nil {
		router.Get("/api/system/metrics", metricsHandler.ServeHTTP)
	}
	router.Get("/api/system/scheduler/jobs", h.HandleSystemSchedulerJobList())
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", h.HandleSystemSchedulerJobTrigger())
}
