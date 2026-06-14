package managementhttp

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func newEnableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Enable
	}
	return newDesiredStateHandler(catalog, repo, "enabled", action, grantRepo, autoGrantProvider)
}

func newDisableHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Disable
	}
	return newDesiredStateHandler(catalog, repo, "disabled", action, grantRepo, autoGrantProvider)
}

type desiredStateAction func(context.Context, string) (plugins.Snapshot, error)

func newDesiredStateHandler(catalog plugins.CatalogView, repo plugins.DesiredStateRepository, desiredState string, action desiredStateAction, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if action != nil {
			snapshot, err := action(r.Context(), pluginID)
			if err == nil {
				writePluginDetailResponse(w, r, catalog, snapshot, grantRepo, autoGrantProvider)
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
			if err := repo.SaveDesiredState(context.Background(), pluginID, desiredState, time.Now().UTC()); err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}
		}
		snapshot, err := catalog.SetDesiredState(pluginID, desiredState)
		if err == nil {
			writePluginDetailResponse(w, r, catalog, snapshot, grantRepo, autoGrantProvider)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newReloadHandler(catalog plugins.CatalogView, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		snapshot, err := controller.Reload(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, r, catalog, snapshot, grantRepo, autoGrantProvider)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func newDeadLetterRecoverHandler(catalog plugins.CatalogView, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		snapshot, err := controller.RecoverFromDeadLetter(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, r, catalog, snapshot, grantRepo, autoGrantProvider)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}

func writePluginDetailResponse(w http.ResponseWriter, r *http.Request, catalog plugins.CatalogView, snapshot plugins.Snapshot, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	response, buildErr := buildPluginDetailResponse(r.Context(), catalog, snapshot, grantRepo, autoGrantProvider)
	if buildErr != nil {
		writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
		return
	}
	writeJSON(w, http.StatusOK, response)
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
