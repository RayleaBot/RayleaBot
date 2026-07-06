package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
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

type SystemService interface {
	CoreService
	SchedulerMetadataService
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
	systemCodePermissionDenied = "permission.denied"
	systemCodeInvalidRequest   = "platform.invalid_request"
	systemCodeResourceMissing  = "platform.resource_missing"
	systemCodeInternalError    = "platform.internal_error"
)

type SystemHTTPError struct {
	statusCode int
	code       string
	message    string
	messageKey string
	details    map[string]any
}

func InternalSystemHTTPError() *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusInternalServerError,
		code:       systemCodeInternalError,
		message:    "内部错误",
		messageKey: "errors.platform.internal_error",
	}
}

func InvalidSystemHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusBadRequest,
		code:       systemCodeInvalidRequest,
		message:    "请求参数不合法",
		messageKey: "errors.platform.invalid_request",
		details:    details,
	}
}

func MissingSystemResourceHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusNotFound,
		code:       systemCodeResourceMissing,
		message:    "缺少必要资源",
		messageKey: "errors.platform.resource_missing",
		details:    details,
	}
}

func WriteSystemHTTPError(w http.ResponseWriter, r *http.Request, err *SystemHTTPError) {
	if err == nil {
		return
	}
	httpapi.WriteError(w, r, err.statusCode, err.code, err.message, err.messageKey, err.details)
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
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		taskID, err := h.system.SubmitSystemBackupTask()
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *SystemHandlers) HandleSystemDiagnostics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, h.system.DiagnosticsSnapshot(r.Context()))
	}
}

func (h *SystemHandlers) HandleSystemDiagnosticsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		archive, err := h.system.BuildDiagnosticsArchive(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
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
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		req, err := decodeRuntimeBootstrapRequest(r)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, systemCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		resources, ok := normalizeRuntimeBootstrapResources(req.Resources)
		if !ok {
			httpapi.WriteError(w, r, http.StatusBadRequest, systemCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		taskID, err := h.system.SubmitRuntimeBootstrapTask(resources)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
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

func (routes SystemRoutes) RegisterProtectedRoutes(router chi.Router) {
	registerSystemProtectedRoutes(router, routes.Handlers, routes.Metrics)
}

func (h *SystemHandlers) RegisterProtectedRoutes(router chi.Router, metricsHandler http.Handler) {
	registerSystemProtectedRoutes(router, h, metricsHandler)
}

func registerSystemProtectedRoutes(router chi.Router, h *SystemHandlers, metricsHandler http.Handler) {
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
