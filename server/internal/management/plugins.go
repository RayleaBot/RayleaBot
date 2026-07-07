package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/go-chi/chi/v5"
)

type PluginRouteDeps struct {
	Catalog      *plugincatalog.Catalog
	TaskRegistry *tasks.Registry
	Repository   plugins.DesiredStateRepository
	Installer    plugins.InstallCoordinator
	Uninstaller  plugins.UninstallCoordinator
	Lifecycle    *pluginservice.Controller
}

const (
	pluginCodeInvalidRequest  = "platform.invalid_request"
	pluginCodeResourceMissing = "platform.resource_missing"
)

type pluginTaskAcceptedResponse struct {
	TaskID string `json:"task_id"`
}

type pluginInstallRequest struct {
	SourceType          string `json:"source_type"`
	Source              string `json:"source"`
	AllowInstallScripts bool   `json:"allow_install_scripts,omitempty"`
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

func RegisterPluginRoutes(router chi.Router, catalog plugins.CatalogView, taskRegistry *tasks.Registry, repo plugins.DesiredStateRepository, installer plugins.InstallCoordinator, controller DesiredStateController, uninstaller UninstallCoordinator) {
	if catalog == nil {
		catalog = emptyCatalogView{}
	}

	registerPluginReadRoutes(router, catalog)
	registerPluginInstallRoutes(router, catalog, taskRegistry, installer)
	registerPluginLifecycleRoutes(router, catalog, repo, controller, uninstaller)
	registerPluginDeadLetterRoutes(router, catalog, controller)
}

func registerPluginReadRoutes(router chi.Router, catalog plugins.CatalogView) {
	router.Get("/api/plugins", newListHandler(catalog))
	router.Get("/api/plugins/{plugin_id}", newDetailHandler(catalog))
}

func registerPluginInstallRoutes(router chi.Router, catalog plugins.CatalogView, taskRegistry *tasks.Registry, installer plugins.InstallCoordinator) {
	router.Post("/api/plugins/install", newInstallHandler(catalog, taskRegistry, installer))
}

func registerPluginLifecycleRoutes(router chi.Router, catalog plugins.CatalogView, repo plugins.DesiredStateRepository, controller DesiredStateController, uninstaller UninstallCoordinator) {
	router.Post("/api/plugins/{plugin_id}/enable", newEnableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/disable", newDisableHandler(catalog, repo, controller))
	router.Post("/api/plugins/{plugin_id}/reload", newReloadHandler(catalog, controller))
	router.Delete("/api/plugins/{plugin_id}", newUninstallHandler(catalog, uninstaller))
}

func registerPluginDeadLetterRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController) {
	router.Post("/api/plugins/{plugin_id}/recover", newDeadLetterRecoverHandler(catalog, controller))
}

func newListHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshots := catalog.List()
		conflicts := plugins.DetectCommandConflicts(snapshots)
		items := make([]SummaryResponse, 0, len(snapshots))
		for _, snapshot := range snapshots {
			items = append(items, ToSummary(snapshot, conflicts[snapshot.PluginID]))
		}

		writeJSON(w, http.StatusOK, ListResponse{Items: items})
	}
}

func newDetailHandler(catalog plugins.CatalogView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(
				w,
				r,
				http.StatusNotFound,
				pluginCodeResourceMissing,
				"缺少必要资源",
				"errors.platform.resource_missing",
				map[string]any{
					"resource_type": "plugin",
					"plugin_id":     pluginID,
				},
			)
			return
		}

		writeJSON(w, http.StatusOK, buildPluginDetailResponse(catalog, snapshot))
	}
}

func buildPluginDetailResponse(catalog plugins.CatalogView, snapshot plugins.Snapshot) DetailResponse {
	return BuildDetail(catalog, snapshot)
}

func newInstallHandler(catalog plugins.CatalogView, taskRegistry *tasks.Registry, installer plugins.InstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pluginInstallRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if (req.SourceType != "local_zip" && req.SourceType != "local_directory" && req.SourceType != "remote_url") || req.Source == "" {
			writeError(w, r, http.StatusBadRequest, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if installer != nil {
			taskID, err := installer.Accept(r.Context(), plugins.InstallRequest{
				SourceType:          req.SourceType,
				Source:              req.Source,
				AllowInstallScripts: req.AllowInstallScripts,
			})
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
				return
			}

			writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
			return
		}

		summary := fmt.Sprintf("install plugin from %s: %s", req.SourceType, req.Source)
		taskID, err := taskRegistry.Create("plugin.install", summary)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
	}
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

func newDeadLetterRecoverHandler(catalog plugins.CatalogView, controller DesiredStateController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
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
	writeJSON(w, http.StatusOK, buildPluginDetailResponse(catalog, snapshot))
}

func newUninstallHandler(catalog plugins.CatalogView, coordinator UninstallCoordinator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		snapshot, ok := catalog.Get(pluginID)
		if !ok {
			writeError(w, r, 404, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
			return
		}
		if snapshot.SourceRoot == "plugins/builtin" {
			writeError(w, r, http.StatusConflict, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
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
		writeJSON(w, http.StatusAccepted, pluginTaskAcceptedResponse{TaskID: taskID})
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

func writeDesiredStateError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	if errors.Is(err, plugins.ErrPluginNotFound) {
		writeError(w, r, 404, pluginCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{"resource_type": "plugin", "plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrPluginNotInDeadLetter) {
		writeError(w, r, 409, "plugin.not_recoverable", "插件当前不可恢复", "errors.plugin.not_recoverable", map[string]any{"plugin_id": pluginID})
		return
	}
	if errors.Is(err, plugins.ErrStateConflict) {
		writeError(w, r, 409, pluginCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", map[string]any{"plugin_id": pluginID})
		return
	}
	writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	MessageKey string         `json:"message_key"`
	RequestID  string         `json:"request_id"`
	Details    map[string]any `json:"details,omitempty"`
}

func (deps PluginRouteDeps) RegisterProtectedRoutes(router chi.Router) {
	RegisterPluginRoutes(
		router,
		deps.Catalog,
		deps.TaskRegistry,
		deps.Repository,
		deps.Installer,
		deps.Lifecycle,
		deps.Uninstaller,
	)
}

type emptyCatalogView struct{}

func (emptyCatalogView) List() []plugins.Snapshot {
	return nil
}

func (emptyCatalogView) Get(string) (plugins.Snapshot, bool) {
	return plugins.Snapshot{}, false
}

func (emptyCatalogView) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, plugins.ErrPluginNotFound
}
