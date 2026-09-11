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
	pluginmarket "github.com/RayleaBot/RayleaBot/server/internal/plugins/market"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type PluginStoreRoutes struct {
	Service pluginmarket.ServiceAPI
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
	Items []pluginmarket.SourceView `json:"items"`
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
		query := pluginmarket.Query{
			SourceID: r.URL.Query().Get("source_id"),
			Text:     r.URL.Query().Get("query"),
			Sort:     r.URL.Query().Get("sort"),
			Limit:    24,
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				writeError(w, r, pluginCodeInvalidRequest, nil)
				return
			}
			query.Cursor = value
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 100 {
				writeError(w, r, pluginCodeInvalidRequest, nil)
				return
			}
			query.Limit = value
		}
		if query.Sort != "" && query.Sort != "recommended" && query.Sort != "name" && query.Sort != "updated" {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		result, err := routes.Service.List(query)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func (routes PluginStoreRoutes) detail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		detail, ok := routes.Service.Get(r.URL.Query().Get("source_id"), pluginID)
		if !ok {
			writeError(w, r, pluginCodeResourceNotFound, map[string]any{"resource_type": "plugin_store_entry", "plugin_id": pluginID})
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func (routes PluginStoreRoutes) inspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request pluginStoreInspectionRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		result, err := routes.Service.Inspect(r.Context(), pluginmarket.InspectionRequest{
			SourceID: strings.TrimSpace(request.SourceID),
			PluginID: chi.URLParam(r, "plugin_id"),
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, pluginStoreInspectionResponse{
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
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		taskID, err := routes.Service.Install(r.Context(), pluginmarket.InstallRequest{
			PluginID:             chi.URLParam(r, "plugin_id"),
			InspectionID:         strings.TrimSpace(request.InspectionID),
			PackageSHA256:        strings.TrimSpace(request.PackageSHA256),
			TrustedCodeConfirmed: request.TrustedCodeConfirmed,
		})
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func (routes PluginStoreRoutes) listSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		items := make([]pluginmarket.SourceView, 0)
		for _, source := range routes.Service.Sources() {
			if pagination.Matches(query.Text, source.ID, source.Name, source.URL) {
				items = append(items, source)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		page, meta := pagination.Slice(items, query)
		writeJSON(w, http.StatusOK, pluginStoreSourcesResponse{Metadata: meta, Items: page})
	}
}

func (routes PluginStoreRoutes) createSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input pluginmarket.SourceInput
		if err := httpapi.DecodeStrictJSON(w, r, &input, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		source, err := routes.Service.CreateSource(r.Context(), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, source)
	}
}

func (routes PluginStoreRoutes) updateSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input pluginmarket.SourceInput
		if err := httpapi.DecodeStrictJSON(w, r, &input, httpapi.MaxManagementJSONBodyBytes); err != nil {
			writeError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		source, err := routes.Service.UpdateSource(r.Context(), chi.URLParam(r, "source_id"), input)
		if err != nil {
			writePluginStoreError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, source)
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
		writeJSON(w, http.StatusOK, source)
	}
}

func writePluginStoreError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pluginmarket.ErrEntryNotFound), errors.Is(err, pluginmarket.ErrSourceNotFound):
		writeError(w, r, pluginCodeResourceNotFound, nil)
	case errors.Is(err, pluginmarket.ErrSourceImmutable):
		writeError(w, r, errorcodes.PluginStoreSourceImmutable, nil)
	case errors.Is(err, pluginmarket.ErrSourceConflict):
		writeError(w, r, errorcodes.PluginStoreSourceConflict, nil)
	case errors.Is(err, pluginmarket.ErrSourceInvalid):
		writeError(w, r, pluginCodeInvalidRequest, nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation),
		errors.Is(err, plugins.ErrInstallInspectionRequired),
		errors.Is(err, plugins.ErrInstallInspectionExpired),
		errors.Is(err, plugins.ErrInstallDigestMismatch),
		errors.Is(err, tasks.ErrQueueFull):
		writePluginInstallError(w, r, err)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeCatalogUnavailable:
		writeError(w, r, pluginmarket.CodeCatalogUnavailable, nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeReleaseUnavailable:
		writeError(w, r, pluginmarket.CodeReleaseUnavailable, nil)
	case pluginmarket.ErrorCode(err) == pluginmarket.CodeIntegrityMismatch || pluginservice.InstallErrorCode(err) == pluginmarket.CodeIntegrityMismatch:
		writeError(w, r, pluginmarket.CodeIntegrityMismatch, nil)
	default:
		writePluginInstallError(w, r, err)
	}
}
