package taskapi

import "github.com/go-chi/chi/v5"

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/tasks", h.HandleTaskList())
	router.Get("/api/tasks/{task_id}", h.HandleTaskDetail())
	router.Post("/api/tasks/{task_id}/cancel", h.HandleTaskCancel())
}
