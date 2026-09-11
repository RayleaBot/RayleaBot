package management

import (
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/market"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type PluginStoreRoutes struct {
	Service market.ServiceAPI
}

type pluginStoreInspectionRequest struct {
	SourceID string `json:"source_id"`
}

type pluginStoreInspectionResponse struct {
	Inspection           pluginInstallInspectionResponse `json:"inspection"`
	ConfirmationRequired bool                            `json:"confirmation_required"`
	ConfirmationReasons  []string                        `json:"confirmation_reasons"`
}

type pluginStoreInstallRequest struct {
	InspectionID         string `json:"inspection_id"`
	PackageSHA256        string `json:"package_sha256"`
	TrustedCodeConfirmed bool   `json:"trusted_code_confirmed"`
}

type pluginStoreSourcesResponse struct {
	pagination.Metadata
	Items []market.SourceView `json:"items"`
}

func (routes PluginStoreRoutes) RegisterProtectedRoutes(router chi.Router) {
	if routes.Service == nil {
		return
	}
	router.Get("/api/plugin-store/plugins", routes.list())
	router.Get("/api/plugin-store/plugins/{plugin_id}", routes.detail())
	router.Post("/api/plugin-store/plugins/{plugin_id}/inspect", routes.inspect())
	router.Post("/api/plugin-store/plugins/{plugin_id}/install", routes.install())
	router.Get("/api/plugin-store/sources", routes.listSources())
	router.Post("/api/plugin-store/sources", routes.createSource())
	router.Put("/api/plugin-store/sources/{source_id}", routes.updateSource())
	router.Delete("/api/plugin-store/sources/{source_id}", routes.deleteSource())
	router.Post("/api/plugin-store/sources/{source_id}/refresh", routes.refreshSource())
}

func (routes PluginStoreRoutes) list() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := market.Query{
			SourceID: r.URL.Query().Get("source_id"),
			Text:     r.URL.Query().Get("query"),
			Sort:     r.URL.Query().Get("sort"),
			Limit:    24,
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
				return
			}
			query.Cursor = value
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 100 {
				httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
				return
			}
			query.Limit = value
		}
		if query.Sort != "" && query.Sort != "recommended" && query.Sort != "name" && query.Sort != "updated" {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		result, err := routes.Service.List(query)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, result)
	}
}

func (routes PluginStoreRoutes) detail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		detail, ok := routes.Service.Get(r.URL.Query().Get("source_id"), pluginID)
		if !ok {
			httpapi.WriteError(w, r, pluginCodeResourceNotFound, map[string]any{"resource_type": "plugin_store_entry", "plugin_id": pluginID})
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, detail)
	}
}

func (routes PluginStoreRoutes) inspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pluginStoreInspectionRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		result, err := routes.Service.Inspect(r.Context(), market.InspectionRequest{
			SourceID: strings.TrimSpace(request.SourceID),
			PluginID: chi.URLParam(r, "plugin_id"),
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, pluginStoreInspectionResponse{
			Inspection:           buildInstallInspectionResponse(result.Inspection),
			ConfirmationRequired: result.ConfirmationRequired,
			ConfirmationReasons:  append([]string(nil), result.ConfirmationReasons...),
		})
	}
}

func (routes PluginStoreRoutes) install() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pluginStoreInstallRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		taskID, err := routes.Service.Install(r.Context(), market.InstallRequest{
			PluginID:             chi.URLParam(r, "plugin_id"),
			InspectionID:         strings.TrimSpace(request.InspectionID),
			PackageSHA256:        strings.TrimSpace(request.PackageSHA256),
			TrustedCodeConfirmed: request.TrustedCodeConfirmed,
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func (routes PluginStoreRoutes) listSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		items := make([]market.SourceView, 0)
		for _, source := range routes.Service.Sources() {
			if pagination.Matches(query.Text, source.ID, source.Name, source.URL) {
				items = append(items, source)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		page, meta := pagination.Slice(items, query)
		httpapi.WriteJSON(w, http.StatusOK, pluginStoreSourcesResponse{Metadata: meta, Items: page})
	}
}

func (routes PluginStoreRoutes) createSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input market.SourceInput
		if err := httpapi.DecodeStrictJSON(w, r, &input, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		source, err := routes.Service.CreateSource(r.Context(), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, source)
	}
}

func (routes PluginStoreRoutes) updateSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input market.SourceInput
		if err := httpapi.DecodeStrictJSON(w, r, &input, httpapi.MaxManagementJSONBodyBytes); err != nil {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		source, err := routes.Service.UpdateSource(r.Context(), chi.URLParam(r, "source_id"), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, source)
	}
}

func (routes PluginStoreRoutes) deleteSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := routes.Service.DeleteSource(r.Context(), chi.URLParam(r, "source_id")); err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (routes PluginStoreRoutes) refreshSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source, err := routes.Service.Refresh(r.Context(), chi.URLParam(r, "source_id"))
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, source)
	}
}

func writePluginStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, market.ErrEntryNotFound), errors.Is(err, market.ErrSourceNotFound):
		httpapi.WriteError(w, r, pluginCodeResourceNotFound, nil)
	case errors.Is(err, market.ErrSourceImmutable):
		httpapi.WriteError(w, r, errorcodes.PluginStoreSourceImmutable, nil)
	case errors.Is(err, market.ErrSourceConflict):
		httpapi.WriteError(w, r, errorcodes.PluginStoreSourceConflict, nil)
	case errors.Is(err, market.ErrSourceInvalid):
		httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation),
		errors.Is(err, plugins.ErrInstallInspectionRequired),
		errors.Is(err, plugins.ErrInstallInspectionExpired),
		errors.Is(err, plugins.ErrInstallDigestMismatch),
		errors.Is(err, tasks.ErrQueueFull):
		writePluginInstallError(w, r, err)
	case market.ErrorCode(err) == market.CodeCatalogUnavailable:
		httpapi.WriteError(w, r, market.CodeCatalogUnavailable, nil)
	case market.ErrorCode(err) == market.CodeReleaseUnavailable:
		httpapi.WriteError(w, r, market.CodeReleaseUnavailable, nil)
	case market.ErrorCode(err) == market.CodeIntegrityMismatch || pluginservice.InstallErrorCode(err) == market.CodeIntegrityMismatch:
		httpapi.WriteError(w, r, market.CodeIntegrityMismatch, nil)
	default:
		writePluginInstallError(w, r, err)
	}
}
