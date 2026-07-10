package management

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

const (
	coreCodePermissionDenied = "permission.denied"
	coreCodeInternalError    = "platform.internal_error"
)

type CoreHandlers struct {
	auth                 coreAuthService
	system               coreSystemService
	requestShutdown      func()
	launcherControlToken *StaticToken
}

type CoreDeps struct {
	Auth                 coreAuthService
	System               coreSystemService
	RequestShutdown      func()
	LauncherControlToken *StaticToken
}

func NewCoreHandlers(deps CoreDeps) *CoreHandlers {
	return &CoreHandlers{
		auth:                 deps.Auth,
		system:               deps.System,
		requestShutdown:      deps.RequestShutdown,
		launcherControlToken: deps.LauncherControlToken,
	}
}

func (h *CoreHandlers) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/setup/status", h.HandleSetupStatus())
	router.Get("/api/launcher/status", h.HandleLauncherStatus())
	router.Post("/api/launcher/shutdown", h.HandleLauncherShutdown())
}

func (h *CoreHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Delete("/api/session", h.HandleSessionLogout())
	router.Get("/api/system/status", h.HandleSystemStatus())
	router.Post("/api/system/shutdown", h.HandleSystemShutdown())
}

type coreAuthService interface {
	IsBootstrapped() bool
	RevokeWithContext(context.Context, string) error
}

type coreSystemService interface {
	StatusSnapshot() systemsvc.StatusSnapshot
	PublishStatusSnapshot()
}

type coreSetupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type CoreSystemStatusResponse struct {
	Status          string                         `json:"status"`
	AdapterState    string                         `json:"adapter_state"`
	ActivePlugins   int                            `json:"active_plugins"`
	RunningPlugins  int                            `json:"running_plugins"`
	FailedPlugins   int                            `json:"failed_plugins"`
	DBSchemaVersion string                         `json:"db_schema_version"`
	UptimeSeconds   int64                          `json:"uptime_seconds"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
	Health          *health.ReadinessReport        `json:"health,omitempty"`
}

type coreShutdownResponse struct {
	Accepted bool `json:"accepted"`
}

func (h *CoreHandlers) HandleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, coreSetupStatusResponse{
			Initialized: h.auth != nil && h.auth.IsBootstrapped(),
		})
	}
}

func (h *CoreHandlers) HandleSessionLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.SessionID == "" {
			writeCoreAuthError(w, r, http.StatusUnauthorized, coreCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if err := h.auth.RevokeWithContext(r.Context(), claims.SessionID); err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, coreCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0).UTC(),
		})

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *CoreHandlers) HandleLauncherStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.validLauncherControlRequest(r) {
			writeCoreAuthError(w, r, http.StatusForbidden, coreCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *CoreHandlers) HandleSystemStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *CoreHandlers) HandleSystemShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		httpapi.WriteJSON(w, http.StatusAccepted, coreShutdownResponse{Accepted: true})
	}
}

func (h *CoreHandlers) HandleLauncherShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.validLauncherControlRequest(r) {
			writeCoreAuthError(w, r, http.StatusForbidden, coreCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		httpapi.WriteJSON(w, http.StatusAccepted, coreShutdownResponse{Accepted: true})
	}
}

func (h *CoreHandlers) validLauncherControlRequest(r *http.Request) bool {
	return isLoopbackRequest(r) && h.launcherControlToken != nil &&
		h.launcherControlToken.Matches(strings.TrimSpace(r.Header.Get(LauncherControlTokenHeader)))
}

func (h *CoreHandlers) writeSystemStatus(w http.ResponseWriter, statusCode int) {
	httpapi.WriteJSON(w, statusCode, coreStatusResponseFromSnapshot(h.system.StatusSnapshot()))
}

func coreStatusResponseFromSnapshot(snapshot systemsvc.StatusSnapshot) CoreSystemStatusResponse {
	return CoreSystemStatusResponse{
		Status:          snapshot.Status,
		AdapterState:    snapshot.AdapterState,
		ActivePlugins:   snapshot.ActivePlugins,
		RunningPlugins:  snapshot.RunningPlugins,
		FailedPlugins:   snapshot.FailedPlugins,
		DBSchemaVersion: snapshot.DBSchemaVersion,
		UptimeSeconds:   snapshot.UptimeSeconds,
		RecoverySummary: snapshot.RecoverySummary,
		Health:          snapshot.Health,
	}
}

func isLoopbackRequest(r *http.Request) bool {
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

func writeCoreAuthError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, nil)
}
