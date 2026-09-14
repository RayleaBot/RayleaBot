package management

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type PluginRouteDeps struct {
	Catalog     *plugincatalog.Catalog
	Installer   plugins.InstallCoordinator
	Uninstaller plugins.UninstallCoordinator
	Lifecycle   *pluginservice.Controller
}

const (
	pluginCodeInvalidRequest   = errorcodes.PlatformInvalidRequest
	pluginCodeResourceNotFound = errorcodes.PlatformResourceNotFound
)

type pluginTaskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

type pluginInstallRequest struct {
	SourceType           string `json:"source_type"`
	Source               string `json:"source"`
	TrustedCodeConfirmed bool   `json:"trusted_code_confirmed"`
}

type DesiredStateController interface {
	Enable(context.Context, string) (plugins.Snapshot, error)
	Disable(context.Context, string) (plugins.Snapshot, error)
	Reload(context.Context, string) (plugins.Snapshot, error)
	RecoverFromDeadLetter(context.Context, string) (plugins.Snapshot, error)
}

type desiredStateAction func(context.Context, string) (plugins.Snapshot, error)

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}

type pluginRoutes struct{ deps PluginRouteDeps }

func NewPluginRoutes(deps PluginRouteDeps) (ProtectedRouteModule, error) {
	if deps.Catalog == nil || deps.Lifecycle == nil || deps.Installer == nil || deps.Uninstaller == nil {
		return nil, errors.New("plugin routes require catalog, lifecycle, installer and uninstaller")
	}
	return pluginRoutes{deps: deps}, nil
}

func (routes pluginRoutes) RegisterProtectedRoutes(router chi.Router) {
	deps := routes.deps
	registerPluginReadRoutes(router, deps.Catalog)
	registerPluginInstallRoutes(router, deps.Catalog, deps.Installer)
	registerPluginLifecycleRoutes(router, deps.Catalog, deps.Lifecycle, deps.Uninstaller)
	registerPluginDeadLetterRoutes(router, deps.Catalog, deps.Lifecycle)
}

func registerPluginReadRoutes(router chi.Router, catalog plugins.CatalogView) {
	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}/icon", newPluginIconHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
}

func registerPluginInstallRoutes(router chi.Router, catalog plugins.CatalogView, installer plugins.InstallCoordinator) {
	router.Post("/api/plugins/install", newInstallHandler(installer))
}

func registerPluginLifecycleRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController, uninstaller UninstallCoordinator) {
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, controller))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, controller))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(uninstaller))
}

func registerPluginDeadLetterRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController) {
	router.Post("/api/plugins/{plugin_id}/recover", newDeadLetterRecoverHandler(catalog, controller))
}

func newListHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := readCollectionQuery(w, r)
		if !ok {
			return
		}
		state, ok := readCollectionChoice(w, r, "state", "running", "disabled", "alert")
		if !ok {
			return
		}
		source, ok := readCollectionChoice(w, r, "source", "official", "community")
		if !ok {
			return
		}
		snapshots := catalog.List()
		conflicts := plugins.DetectCommandConflicts(snapshots)
		page, meta := plugins.ListPage(snapshots, conflicts, plugins.ListFilter{State: state, Source: source}, query)
		items := make([]SummaryResponse, 0, len(page))
		for _, snapshot := range page {
			items = append(items, toSummary(snapshot, conflicts[snapshot.PluginID]))
		}
		httpapi.WriteJSON(w, http.StatusOK, ListResponse{Metadata: meta, Items: items})
	}
}

func newDetailHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			httpapi.WriteError(
				w,
				r,

				pluginCodeResourceNotFound,

				map[string]any{
					"resource_type": "plugin",
					"plugin_id":     pluginID,
				},
			)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, buildDetail(catalog, snapshot))
	}
}

func newInstallHandler(installer plugins.InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallRequest
		if err := httpapi.DecodeStrictJSON(w, r, &req, httpapi.MaxManagementJSONBodyBytes); err != nil || !validPluginInstallSource(req.SourceType, req.Source) {
			httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
			return
		}
		if !req.TrustedCodeConfirmed {
			writePluginInstallError(w, r, plugins.ErrTrustedCodeConfirmation)
			return
		}

		if installer != nil {
			taskID, err := installer.Accept(r.Context(), plugins.InstallRequest{
				SourceType:           req.SourceType,
				Source:               req.Source,
				TrustedCodeRequired:  true,
				TrustedCodeConfirmed: true,
			})
			if err != nil {
				writePluginInstallError(w, r, err)
				return
			}

			httpapi.WriteJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
			return
		}

		httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
	}
}

func validPluginInstallSource(sourceType, source string) bool {
	return (sourceType == "local_zip" || sourceType == "local_directory" || sourceType == "remote_url") && strings.TrimSpace(source) != ""
}

func writePluginInstallError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tasks.ErrQueueFull):
		httpapi.WriteError(w, r, errorcodes.PlatformTaskQueueFull, nil)
	case errors.Is(err, plugins.ErrTrustedCodeConfirmation):
		httpapi.WriteError(w, r, errorcodes.PluginTrustedCodeConfirmationRequired, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPackageResourceLimitExceeded:
		httpapi.WriteError(w, r, errorcodes.PluginPackageResourceLimitExceeded, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPackageUnsafeEntry:
		httpapi.WriteError(w, r, errorcodes.PluginPackageUnsafeEntry, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginArtifactInvalid:
		httpapi.WriteError(w, r, errorcodes.PluginArtifactInvalid, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginPlatformMismatch:
		httpapi.WriteError(w, r, errorcodes.PluginPlatformMismatch, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PluginStoreIntegrityMismatch:
		httpapi.WriteError(w, r, errorcodes.PluginStoreIntegrityMismatch, nil)
	case pluginservice.InstallErrorCode(err) == errorcodes.PlatformInvalidRequest || pluginservice.InstallErrorCode(err) == errorcodes.PlatformResourceMissing:
		httpapi.WriteError(w, r, pluginCodeInvalidRequest, nil)
	default:
		httpapi.WriteError(w, r, errorcodes.PluginInstallFailed, nil)
	}
}

func newEnableHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return newDesiredStateHandler(catalog, controller.Enable)
}

func newDisableHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return newDesiredStateHandler(catalog, controller.Disable)
}

func newDesiredStateHandler(catalog plugins.CatalogView, action desiredStateAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, err := action(r.Context(), pluginID)
		if err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		writePluginDetailResponse(w, catalog, snapshot)
	}
}

func newReloadHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		snapshot, err := controller.Reload(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newDeadLetterRecoverHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		snapshot, err := controller.RecoverFromDeadLetter(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func writePluginDetailResponse(w http.ResponseWriter, catalog plugins.CatalogView, snapshot plugins.Snapshot) {
	httpapi.WriteJSON(w, http.StatusOK, buildDetail(catalog, snapshot))
}

func newUninstallHandler(coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if !plugins.ValidPluginID(pluginID) {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}
		if coordinator == nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		taskID, err := coordinator.Accept(r.Context(), pluginID)
		if err != nil {
			if errors.Is(err, plugins.ErrInvalidPluginID) {
				httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
				return
			}
			if errors.Is(err, tasks.ErrQueueFull) {
				httpapi.WriteError(w, r, errorcodes.PlatformTaskQueueFull, nil)
				return
			}
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
}

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, plugins.ErrPluginNotFound) {
		httpapi.WriteError(w, r, pluginCodeResourceNotFound, map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrPluginNotInDeadLetter) {
		httpapi.WriteError(w, r, errorcodes.PluginNotRecoverable, map[string]any{"plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrStateConflict) {
		httpapi.WriteError(w, r, errorcodes.PlatformStateConflict, map[string]any{"plugin_id": pluginID})
		return
	}
	httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
}
