package coreapi

import (
	"net"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
)

const (
	codePermissionDenied = "permission.denied"
	codeInternalError    = "platform.internal_error"
)

type Handlers struct {
	auth            authService
	system          systemService
	requestShutdown func()
}

type Deps struct {
	Auth            authService
	System          systemService
	RequestShutdown func()
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		auth:            deps.Auth,
		system:          deps.System,
		requestShutdown: deps.RequestShutdown,
	}
}

func (h *Handlers) SetAuthManager(auth authService) {
	if h == nil {
		return
	}
	h.auth = auth
}

type authService interface {
	IsBootstrapped() bool
	Revoke(string) error
}

type systemService interface {
	StatusSnapshot() systemmodel.StatusSnapshot
	PublishStatusSnapshot()
}

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type SystemStatusResponse struct {
	Status          string                         `json:"status"`
	AdapterState    string                         `json:"adapter_state"`
	ActivePlugins   int                            `json:"active_plugins"`
	RunningPlugins  int                            `json:"running_plugins"`
	FailedPlugins   int                            `json:"failed_plugins"`
	DBSchemaVersion string                         `json:"db_schema_version"`
	UptimeSeconds   int64                          `json:"uptime_seconds"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

type shutdownResponse struct {
	Accepted bool `json:"accepted"`
}

func (h *Handlers) HandleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, setupStatusResponse{
			Initialized: h.auth != nil && h.auth.IsBootstrapped(),
		})
	}
}

func (h *Handlers) HandleSessionLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := authhttp.ClaimsFromContext(r.Context())
		if !ok || claims.SessionID == "" {
			writeAuthError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if err := h.auth.Revoke(claims.SessionID); err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handlers) HandleLauncherStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *Handlers) HandleSystemStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *Handlers) HandleSystemShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		writeAuthJSON(w, http.StatusAccepted, shutdownResponse{Accepted: true})
	}
}

func (h *Handlers) HandleLauncherShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		writeAuthJSON(w, http.StatusAccepted, shutdownResponse{Accepted: true})
	}
}

func (h *Handlers) writeSystemStatus(w http.ResponseWriter, statusCode int) {
	writeAuthJSON(w, statusCode, systemStatusResponseFromSnapshot(h.system.StatusSnapshot()))
}

func systemStatusResponseFromSnapshot(snapshot systemmodel.StatusSnapshot) SystemStatusResponse {
	return SystemStatusResponse{
		Status:          snapshot.Status,
		AdapterState:    snapshot.AdapterState,
		ActivePlugins:   snapshot.ActivePlugins,
		RunningPlugins:  snapshot.RunningPlugins,
		FailedPlugins:   snapshot.FailedPlugins,
		DBSchemaVersion: snapshot.DBSchemaVersion,
		UptimeSeconds:   snapshot.UptimeSeconds,
		RecoverySummary: snapshot.RecoverySummary,
	}
}

func IsLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if hasForwardingHeaders(r) {
		return false
	}

	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return false
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func hasForwardingHeaders(r *http.Request) bool {
	for _, header := range []string{
		"Forwarded",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Port",
		"X-Forwarded-Proto",
		"X-Real-IP",
	} {
		if strings.TrimSpace(r.Header.Get(header)) != "" {
			return true
		}
	}

	return false
}

func writeAuthError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string) {
	writeAppError(w, r, statusCode, code, message, messageKey, nil)
}

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
