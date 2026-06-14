package governanceapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func decodeGovernanceEntryUpsertRequest(w http.ResponseWriter, r *http.Request) (governanceEntryUpsertRequest, bool) {
	var request governanceEntryUpsertRequest
	if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	request.EntryType = strings.TrimSpace(request.EntryType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Reason = strings.TrimSpace(request.Reason)
	if !governance.IsEntryType(request.EntryType) || request.TargetID == "" || request.Reason == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	return request, true
}

func readGovernanceEntryPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	entryType := strings.TrimSpace(chi.URLParam(r, "entry_type"))
	targetID := strings.TrimSpace(chi.URLParam(r, "target_id"))
	if !governance.IsEntryType(entryType) || targetID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return "", "", false
	}
	return entryType, targetID, true
}
