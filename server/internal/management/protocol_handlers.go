package management

import (
	"context"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type protocolAcceptedResponse struct {
	Accepted bool `json:"accepted"`
}

const protocolCodeInvalidRequest = errorcodes.PlatformInvalidRequest

type oneBot11IdentityResolveRequest struct {
	Items []adapterservice.OneBot11IdentityResolveItem `json:"items"`
}

type ProtocolHandlers struct {
	protocol protocolHTTPService
}

type protocolHTTPService interface {
	Adapters() adapterservice.AdaptersView
	CurrentOneBot11ProtocolTargets(context.Context, string) (adapterservice.OneBot11ProtocolTargets, error)
	ResolveOneBot11Identities(context.Context, string, []adapterservice.OneBot11IdentityResolveItem) (adapterservice.OneBot11IdentityResolveResult, error)
	CurrentOneBot11ProtocolCompatibility() (adapterservice.OneBot11ProtocolCompatibility, error)
	OneBot11Ingress(id string) (adapterservice.OneBot11Ingress, bool)
}

func NewProtocolHandlers(protocol protocolHTTPService) *ProtocolHandlers {
	return &ProtocolHandlers{protocol: protocol}
}

func (h *ProtocolHandlers) RegisterPublicRoutes(router chi.Router) {
	// Ingress is addressed per adapter instance: several OneBot adapters can be
	// listening at once, each with its own credential.
	router.Get("/api/adapters/{adapterID}/reverse-ws", h.HandleAdapterReverseWS())
	router.Post("/api/adapters/{adapterID}/webhook", h.HandleAdapterWebhook())
}

func (h *ProtocolHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/adapters", h.HandleAdapters())
	router.Get("/api/adapters/{adapterID}/onebot11/targets", h.HandleProtocolOneBot11Targets())
	router.Post("/api/adapters/{adapterID}/onebot11/identities/resolve", h.HandleProtocolOneBot11IdentitiesResolve())
	router.Get("/api/protocols/onebot11/compatibility", h.HandleProtocolOneBot11Compatibility())
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Targets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.CurrentOneBot11ProtocolTargets(r.Context(), chi.URLParam(r, "adapterID"))
		if err != nil {
			httpapi.WriteError(w, r, protocolCodeInvalidRequest, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11IdentitiesResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body oneBot11IdentityResolveRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || len(body.Items) == 0 || len(body.Items) > 100 {
			httpapi.WriteError(w, r, protocolCodeInvalidRequest, nil)
			return
		}
		response, err := h.protocol.ResolveOneBot11Identities(r.Context(), chi.URLParam(r, "adapterID"), body.Items)
		if err != nil {
			httpapi.WriteError(w, r, protocolCodeInvalidRequest, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleProtocolOneBot11Compatibility() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := h.protocol.CurrentOneBot11ProtocolCompatibility()
		if err != nil {
			httpapi.WriteError(w, r, errorcodes.AdapterMatrixProjectionFailed, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ProtocolHandlers) HandleAdapterReverseWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ingress, ok := h.protocol.OneBot11Ingress(chi.URLParam(r, "adapterID"))
		if !ok {
			httpapi.WriteError(w, r, errorcodes.AdapterTransportReverseWsUpgradeFailed, nil)
			return
		}
		if !ingress.ReverseWSEnabled() {
			httpapi.WriteError(w, r, errorcodes.AdapterTransportReverseWsUpgradeFailed, nil)
			return
		}
		if !allowOneBotIngress(r, ingress.ReverseWSAccessToken(), ingress.ReverseWSAccessTokenQueryCompat()) {
			ingress.MarkReverseWSAuthFailed()
			httpapi.WriteError(w, r, errorcodes.AdapterTransportReverseWsAuthFailed, nil)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		ingress.AttachReverseWS(conn)
	}
}

func (h *ProtocolHandlers) HandleAdapterWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ingress, ok := h.protocol.OneBot11Ingress(chi.URLParam(r, "adapterID"))
		if !ok {
			httpapi.WriteError(w, r, errorcodes.AdapterTransportUnavailable, nil)
			return
		}
		if !ingress.WebhookEnabled() {
			httpapi.WriteError(w, r, errorcodes.AdapterTransportUnavailable, nil)
			return
		}
		if !allowOneBotIngress(r, ingress.WebhookAccessToken(), ingress.WebhookAccessTokenQueryCompat()) {
			ingress.MarkWebhookAuthFailed()
			httpapi.WriteError(w, r, errorcodes.AdapterTransportWebhookAuthFailed, nil)
			return
		}

		payload, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			httpapi.WriteError(w, r, protocolCodeInvalidRequest, nil)
			return
		}
		if err := ingress.AcceptWebhookPayload(r.Context(), payload); err != nil {
			httpapi.WriteError(w, r, errorcodes.AdapterTransportWebhookInvalidPayload, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusAccepted, protocolAcceptedResponse{Accepted: true})
	}
}

func allowOneBotIngress(r *http.Request, accessToken string, allowQueryToken bool) bool {
	trimmedToken := strings.TrimSpace(accessToken)
	if trimmedToken == "" {
		return true
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		if strings.TrimSpace(authHeader[7:]) == trimmedToken {
			return true
		}
	}
	if allowQueryToken && strings.TrimSpace(r.URL.Query().Get("access_token")) == trimmedToken {
		return true
	}
	return false
}

// HandleAdapters lists the configured adapter instances alongside the
// protocols an instance can be added for, so the management surface can show
// what is connected and offer the rest as something to add.
func (h *ProtocolHandlers) HandleAdapters() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, h.protocol.Adapters())
	}
}
