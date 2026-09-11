package management

import (
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (h *SystemHandlers) HandleTaskStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, ok := h.system.GetTaskStatus(chi.URLParam(r, "task_id"))
		if !ok {
			writeSystemHTTPError(w, r, missingSystemResourceHTTPError(nil))
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		httpapi.WriteJSON(w, http.StatusOK, status)
	}
}
