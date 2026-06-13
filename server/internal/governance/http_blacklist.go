package governance

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *Handlers) handleGovernanceBlacklist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := h.service.ReadBlacklist(r.Context())
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, snapshot)
	}
}

func (h *Handlers) handleGovernanceBlacklistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		entry, err := h.service.UpsertBlacklistEntry(r.Context(), request.EntryType, request.TargetID, request.Reason)
		if err != nil {
			writeGovernanceError(w, r, err, request.EntryType, request.TargetID)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, entry)
	}
}

func (h *Handlers) handleGovernanceBlacklistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.service.DeleteBlacklistEntry(r.Context(), entryType, targetID); err != nil {
			writeGovernanceError(w, r, err, entryType, targetID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
