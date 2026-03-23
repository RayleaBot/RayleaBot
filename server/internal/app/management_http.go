package app

import (
	"net/http"
	"time"
)

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type launcherTokenResponse struct {
	LauncherToken string `json:"launcher_token"`
}

type systemStatusResponse struct {
	Status        string `json:"status"`
	AdapterState  string `json:"adapter_state"`
	ActivePlugins int    `json:"active_plugins"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

type systemShutdownResponse struct {
	Accepted bool `json:"accepted"`
}

func (a *App) handleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, setupStatusResponse{
			Initialized: a.Auth != nil && a.Auth.IsBootstrapped(),
		})
	}
}

func (a *App) handleSessionLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.SessionID == "" {
			writeAuthError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if err := a.Auth.Revoke(claims.SessionID); err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *App) handleLauncherTokenIssue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if a.Auth == nil || !a.Auth.IsBootstrapped() {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		token, err := a.launcherTokens.Issue()
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, launcherTokenResponse{LauncherToken: token})
	}
}

func (a *App) handleSystemStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, systemStatusResponse{
			Status:        a.systemStatus(),
			AdapterState:  string(stateOrIdle(a.Adapter.Snapshot().State)),
			ActivePlugins: a.activePluginCount(),
			UptimeSeconds: a.uptimeSeconds(),
		})
	}
}

func (a *App) handleSystemShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		a.requestShutdown()
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}

func (a *App) activePluginCount() int {
	if a == nil || a.Runtimes == nil {
		return 0
	}
	return a.Runtimes.ActiveCount()
}

func (a *App) uptimeSeconds() int64 {
	if a == nil || a.startedAt.IsZero() {
		return 0
	}

	uptime := time.Since(a.startedAt)
	if uptime < 0 {
		return 0
	}

	return int64(uptime / time.Second)
}

func (a *App) systemStatus() string {
	if a != nil && a.shuttingDown.Load() {
		return "shutting_down"
	}

	return "running"
}
