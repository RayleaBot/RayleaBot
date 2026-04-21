package governance

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type governanceEntryResponse = EntryResponse
type governanceBlacklistResponse = BlacklistSnapshot
type governanceWhitelistResponse = WhitelistSnapshot
type governanceWhitelistStateResponse = WhitelistStateResponse
type governanceCommandCooldownResponse = CommandCooldownResponse
type governanceCommandPolicyEntryResponse = CommandPolicyEntryResponse
type governanceCommandPolicyResponse = CommandPolicyResponse

type Handlers struct {
	service *Service
}

type governanceEntryUpsertRequest struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
}

type governanceWhitelistStateUpdateRequest struct {
	Enabled *bool `json:"enabled"`
}

func NewHandlers(deps Deps) *Handlers {
	return NewHandlersWithService(NewService(deps))
}

func NewHandlersWithService(service *Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/governance/blacklist", h.handleGovernanceBlacklist())
	router.Post("/api/governance/blacklist/entries", h.handleGovernanceBlacklistEntryUpsert())
	router.Delete("/api/governance/blacklist/entries/{entry_type}/{target_id}", h.handleGovernanceBlacklistEntryDelete())
	router.Get("/api/governance/whitelist", h.handleGovernanceWhitelist())
	router.Put("/api/governance/whitelist/state", h.handleGovernanceWhitelistStatePut())
	router.Post("/api/governance/whitelist/entries", h.handleGovernanceWhitelistEntryUpsert())
	router.Delete("/api/governance/whitelist/entries/{entry_type}/{target_id}", h.handleGovernanceWhitelistEntryDelete())
	router.Get("/api/governance/command-policy", h.handleGovernanceCommandPolicy())
}

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

func decodeGovernanceEntryUpsertRequest(w http.ResponseWriter, r *http.Request) (governanceEntryUpsertRequest, bool) {
	var request governanceEntryUpsertRequest
	if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	request.EntryType = strings.TrimSpace(request.EntryType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Reason = strings.TrimSpace(request.Reason)
	if !IsEntryType(request.EntryType) || request.TargetID == "" || request.Reason == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return governanceEntryUpsertRequest{}, false
	}

	return request, true
}

func readGovernanceEntryPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	entryType := strings.TrimSpace(chi.URLParam(r, "entry_type"))
	targetID := strings.TrimSpace(chi.URLParam(r, "target_id"))
	if !IsEntryType(entryType) || targetID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
		return "", "", false
	}
	return entryType, targetID, true
}

func writeGovernanceError(w http.ResponseWriter, r *http.Request, err error, entryType, targetID string) {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
			"entry_type": entryType,
			"target_id":  targetID,
		})
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
	}
}
