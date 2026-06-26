package managementui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type Deps struct {
	Plugins            plugins.CatalogView
	PluginConfig       pluginconfig.Repository
	Secrets            secrets.Store
	NotifyConfigChange func(context.Context, string)
	RefreshCommands    func(context.Context, string, map[string]any)
	ActionInvoker      ManagementActionInvoker
}

type Handlers struct {
	plugins            plugins.CatalogView
	pluginConfig       pluginconfig.Repository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
	actionInvoker      ManagementActionInvoker
}

type ManagementActionInvoker interface {
	InvokeManagementAction(context.Context, string, string, map[string]any) (map[string]any, error)
}

type pluginManagementActionRequest struct {
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload,omitempty"`
}

type PluginManagementActionResponse struct {
	PluginID string         `json:"plugin_id"`
	Action   string         `json:"action"`
	Result   map[string]any `json:"result"`
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		plugins:            deps.Plugins,
		pluginConfig:       deps.PluginConfig,
		secrets:            deps.Secrets,
		notifyConfigChange: deps.NotifyConfigChange,
		refreshCommands:    deps.RefreshCommands,
		actionInvoker:      deps.ActionInvoker,
	}
}

func (h *Handlers) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
	router.Head("/plugin-ui/{plugin_id}/*", h.HandlePluginManagementUIStatic())
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsGet())
	router.Put("/api/plugins/{plugin_id}/settings", h.HandlePluginSettingsPut())
	router.Get("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsGet())
	router.Put("/api/plugins/{plugin_id}/secrets", h.HandlePluginSecretsPut())
	router.Post("/api/plugins/{plugin_id}/management/actions", h.HandlePluginManagementAction())
}

func (h *Handlers) HandlePluginManagementAction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := strings.TrimSpace(chi.URLParam(r, "plugin_id"))
		actionInvoker := h.actionInvoker
		if pluginID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if _, ok := h.resolveSettingsSnapshot(w, r); !ok {
			return
		}
		if actionInvoker == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		var request pluginManagementActionRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		action := strings.TrimSpace(request.Action)
		if action == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if request.Payload == nil {
			request.Payload = map[string]any{}
		}

		result, err := actionInvoker.InvokeManagementAction(r.Context(), pluginID, action, request.Payload)
		if err != nil {
			httpapi.WriteDomainError(w, r, &httpapi.DomainError{
				Code:        "plugin.internal_error",
				HTTPStatus:  http.StatusBadGateway,
				SafeMessage: "插件操作失败",
				MessageKey:  "errors.plugin.internal_error",
				Cause:       err,
			})
			return
		}
		if result == nil {
			result = map[string]any{}
		}
		httpapi.WriteJSON(w, http.StatusOK, PluginManagementActionResponse{
			PluginID: pluginID,
			Action:   action,
			Result:   result,
		})
	}
}
