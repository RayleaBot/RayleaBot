package management

import (
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h *SystemHandlers) HandleTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, ok := h.system.GetTaskStatus(chi.URLParam(r, "task_id"))
		if !ok {
			WriteSystemHTTPError(w, r, MissingSystemResourceHTTPError(nil))
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}
