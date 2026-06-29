package systemapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Routes struct {
	Handlers *SystemHandlers
	Metrics  http.Handler
}

type ModuleDeps struct {
	System    CoreService
	Scheduler SchedulerEngineService
	Metrics   http.Handler
}

func NewRoutes(handlers *SystemHandlers, metrics http.Handler) Routes {
	return Routes{Handlers: handlers, Metrics: metrics}
}

func NewModule(deps ModuleDeps) Routes {
	handlers := NewSystemHandlers(deps.System)
	if deps.Scheduler != nil {
		handlers = NewSystemHandlers(deps.System, deps.Scheduler)
	}
	return NewRoutes(handlers, deps.Metrics)
}

func (routes Routes) RegisterProtectedRoutes(router chi.Router) {
	registerProtectedRoutes(router, routes.Handlers, routes.Metrics)
}

func (h *SystemHandlers) RegisterProtectedRoutes(router chi.Router, metricsHandler http.Handler) {
	registerProtectedRoutes(router, h, metricsHandler)
}

func registerProtectedRoutes(router chi.Router, h *SystemHandlers, metricsHandler http.Handler) {
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
