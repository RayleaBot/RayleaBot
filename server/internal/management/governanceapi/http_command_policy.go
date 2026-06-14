package governanceapi

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *Handlers) handleGovernanceCommandPolicy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.service.ReadCommandPolicy(r.Context())
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}
