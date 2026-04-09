package app

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type launcherTokenResponse struct {
	LauncherToken string `json:"launcher_token"`
}

type systemStatusResponse struct {
	Status          string                         `json:"status"`
	AdapterState    string                         `json:"adapter_state"`
	ActivePlugins   int                            `json:"active_plugins"`
	UptimeSeconds   int64                          `json:"uptime_seconds"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

type systemShutdownResponse struct {
	Accepted bool `json:"accepted"`
}

func (h *managementHTTPHandlers) handleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, setupStatusResponse{
			Initialized: h.auth != nil && h.auth.IsBootstrapped(),
		})
	}
}

func (h *managementHTTPHandlers) handleSessionLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
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

func (h *managementHTTPHandlers) handleLauncherTokenIssue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if h.auth == nil || !h.auth.IsBootstrapped() {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		token, err := h.launcherTokens.Issue()
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, launcherTokenResponse{LauncherToken: token})
	}
}

func (h *managementHTTPHandlers) handleSystemStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, systemStatusResponse{
			Status:          h.system.systemStatus(),
			AdapterState:    string(stateOrIdle(h.system.adapter.Snapshot().State)),
			ActivePlugins:   h.system.activePluginCount(),
			UptimeSeconds:   h.system.uptimeSeconds(),
			RecoverySummary: h.system.state.recoverySummarySnapshot(),
		})
	}
}

func (h *managementHTTPHandlers) handleSystemShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.requestShutdown()
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}
