package pluginapi

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

type DesiredStateController interface {
	Enable(context.Context, string) (plugins.Snapshot, error)
	Disable(context.Context, string) (plugins.Snapshot, error)
	Reload(context.Context, string) (plugins.Snapshot, error)
	RecoverFromDeadLetter(context.Context, string) (plugins.Snapshot, error)
}

func registerPluginLifecycleRoutes(router chi.Router, catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController, uninstaller UninstallCoordinator) {
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
}

func newEnableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Enable
	}
	return newDesiredStateHandler(catalog, repo, "enabled", action)
}

func newDisableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Disable
	}
	return newDesiredStateHandler(catalog, repo, "disabled", action)
}

type desiredStateAction func(context.Context, string) (plugins.Snapshot, error)

func newDesiredStateHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, desiredState string, action desiredStateAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if action != nil {
			snapshot, err := action(r.Context(), pluginID)
			if err == nil {
				writePluginDetailResponse(w, catalog, snapshot)
				return
			}
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if err := validateDesiredStateChange(catalog, pluginID, desiredState); err != nil {
			writeDesiredStateError(w, r, pluginID, err)
			return
		}
		if repo != nil {
			if err := repo.SaveDesiredState(r.Context(), pluginID, desiredState, time.Now().UTC()); err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
		}
		snapshot, err := catalog.SetDesiredState(pluginID, desiredState)
		if err == nil {
			writePluginDetailResponse(w, catalog, snapshot)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newReloadHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
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

func writePluginDetailResponse(w http.ResponseWriter, catalog plugins.CatalogView, snapshot plugins.Snapshot) {
	writeJSON(w, http.StatusOK, buildPluginDetailResponse(catalog, snapshot))
}

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}

func newUninstallHandler(catalog plugins.CatalogView, coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if snapshot.SourceRoot == "plugins/builtin" {
			writeError(w, r, http.StatusConflict, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
			return
		}
		if coordinator == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		taskID, err := coordinator.Accept(r.Context(), pluginID)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func validateDesiredStateChange(catalog plugins.CatalogView, pluginID string, desired string) error {
	snapshot, ok := catalog.Get(pluginID)
	if !ok {
		return plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return plugins.ErrStateConflict
	}
	if snapshot.DesiredState == desired {
		return plugins.ErrStateConflict
	}
	return nil
}
