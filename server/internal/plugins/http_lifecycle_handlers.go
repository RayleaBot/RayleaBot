package plugins

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func newEnableHandler(catalog *Catalog, repo DesiredStateRepository, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Enable
	}
	return newDesiredStateHandler(catalog, repo, "enabled", action, grantRepo, autoGrantProvider)
}

func newDisableHandler(catalog *Catalog, repo DesiredStateRepository, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	var action desiredStateAction
	if controller != nil {
		action = controller.Disable
	}
	return newDesiredStateHandler(catalog, repo, "disabled", action, grantRepo, autoGrantProvider)
}

type desiredStateAction func(context.Context, string) (Snapshot, error)

func newDesiredStateHandler(catalog *Catalog, repo DesiredStateRepository, desiredState string, action desiredStateAction, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
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

func newReloadHandler(catalog *Catalog, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
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

func newDeadLetterRecoverHandler(catalog *Catalog, controller DesiredStateController, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
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

func writePluginDetailResponse(w http.ResponseWriter, r *http.Request, catalog *Catalog, snapshot Snapshot, grantRepo GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
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

func newUninstallHandler(catalog *Catalog, coordinator UninstallCoordinator) http.HandlerFunc {
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

func validateDesiredStateChange(catalog *Catalog, pluginID string, desired string) error {
	snapshot, ok := catalog.Get(pluginID)
	if !ok {
		return ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return ErrStateConflict
	}
	if snapshot.DesiredState == desired {
		return ErrStateConflict
	}
	return nil
}
