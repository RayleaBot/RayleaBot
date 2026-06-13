package governance

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *Handlers) handleGovernanceWhitelist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := h.service.ReadWhitelist(r.Context())
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, snapshot)
	}
}

func (h *Handlers) handleGovernanceWhitelistStatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request governanceWhitelistStateUpdateRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Enabled == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		response, err := h.service.SetWhitelistEnabled(r.Context(), *request.Enabled)
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *Handlers) handleGovernanceWhitelistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		entry, err := h.service.UpsertWhitelistEntry(r.Context(), request.EntryType, request.TargetID, request.Reason)
		if err != nil {
			writeGovernanceError(w, r, err, request.EntryType, request.TargetID)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, entry)
	}
}

func (h *Handlers) handleGovernanceWhitelistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.service.DeleteWhitelistEntry(r.Context(), entryType, targetID); err != nil {
			writeGovernanceError(w, r, err, entryType, targetID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
