package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
)

type GovernanceHandlers struct {
	service *governance.Service
}

func NewGovernanceHandlers(deps governance.Deps) *GovernanceHandlers {
	return NewGovernanceHandlersWithService(governance.NewService(deps))
}

func NewGovernanceHandlersWithService(service *governance.Service) *GovernanceHandlers {
	return &GovernanceHandlers{service: service}
}

func (h *GovernanceHandlers) RegisterProtectedRoutes(router chi.Router) {
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

func (h *GovernanceHandlers) handleGovernanceCommandPolicy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.service.ReadCommandPolicy(r.Context())
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

type governanceEntryUpsertRequest struct {
	Scope     chatevent.IdentityScope `json:"scope"`
	EntryType string                  `json:"entry_type"`
	TargetID  string                  `json:"target_id"`
	Reason    string                  `json:"reason"`
}

type governanceWhitelistStateUpdateRequest struct {
	Enabled *bool `json:"enabled"`
}

func (h *GovernanceHandlers) handleGovernanceBlacklist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		snapshot, err := h.service.ReadBlacklistPage(r.Context(), query, r.URL.Query().Get("entry_type"))
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, snapshot)
	}
}

func (h *GovernanceHandlers) handleGovernanceBlacklistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		entry, err := h.service.UpsertBlacklistEntry(r.Context(), request.Scope, request.EntryType, request.TargetID, request.Reason)
		if err != nil {
			writeGovernanceError(w, r, err, request.EntryType, request.TargetID)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, entry)
	}
}

func (h *GovernanceHandlers) handleGovernanceBlacklistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.service.DeleteBlacklistEntry(r.Context(), scope, entryType, targetID); err != nil {
			writeGovernanceError(w, r, err, entryType, targetID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *GovernanceHandlers) handleGovernanceWhitelist() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		snapshot, err := h.service.ReadWhitelistPage(r.Context(), query, r.URL.Query().Get("entry_type"))
		if err != nil {
			writeGovernanceError(w, r, err, "", "")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, snapshot)
	}
}

func (h *GovernanceHandlers) handleGovernanceWhitelistStatePut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request governanceWhitelistStateUpdateRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Enabled == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
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

func (h *GovernanceHandlers) handleGovernanceWhitelistEntryUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeGovernanceEntryUpsertRequest(w, r)
		if !ok {
			return
		}

		entry, err := h.service.UpsertWhitelistEntry(r.Context(), request.Scope, request.EntryType, request.TargetID, request.Reason)
		if err != nil {
			writeGovernanceError(w, r, err, request.EntryType, request.TargetID)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, entry)
	}
}

func (h *GovernanceHandlers) handleGovernanceWhitelistEntryDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, entryType, targetID, ok := readGovernanceEntryPath(w, r)
		if !ok {
			return
		}

		if err := h.service.DeleteWhitelistEntry(r.Context(), scope, entryType, targetID); err != nil {
			writeGovernanceError(w, r, err, entryType, targetID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func decodeGovernanceEntryUpsertRequest(w http.ResponseWriter, r *http.Request) (governanceEntryUpsertRequest, bool) {
	var request governanceEntryUpsertRequest
	if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
		return governanceEntryUpsertRequest{}, false
	}

	request.EntryType = strings.TrimSpace(request.EntryType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Reason = strings.TrimSpace(request.Reason)
	if !request.Scope.Valid() || !governance.IsEntryType(request.EntryType) || request.TargetID == "" || request.Reason == "" {
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
		return governanceEntryUpsertRequest{}, false
	}

	return request, true
}

func readGovernanceEntryPath(w http.ResponseWriter, r *http.Request) (chatevent.IdentityScope, string, string, bool) {
	scope := chatevent.IdentityScope{Kind: r.URL.Query().Get("kind"), SourceProtocol: r.URL.Query().Get("source_protocol"), SourceAdapter: r.URL.Query().Get("source_adapter"), BotID: r.URL.Query().Get("bot_id")}
	entryType := strings.TrimSpace(chi.URLParam(r, "entry_type"))
	targetID := strings.TrimSpace(chi.URLParam(r, "target_id"))
	if !scope.Valid() || !governance.IsEntryType(entryType) || targetID == "" {
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
		return scope, "", "", false
	}
	return scope, entryType, targetID, true
}

func writeGovernanceError(w http.ResponseWriter, r *http.Request, err error, entryType, targetID string) {
	switch {
	case errors.Is(err, governance.ErrInvalidRequest):
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
	case errors.Is(err, permission.ErrGovernanceEntryNotFound):
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
			"entry_type": entryType,
			"target_id":  targetID,
		})
	default:
		httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
	}
}
