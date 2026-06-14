package managementhttp

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

type setupStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type SystemStatusResponse struct {
	Status          string                         `json:"status"`
	AdapterState    string                         `json:"adapter_state"`
	ActivePlugins   int                            `json:"active_plugins"`
	UptimeSeconds   int64                          `json:"uptime_seconds"`
	RecoverySummary *recovery.CompatibilitySummary `json:"recovery_summary,omitempty"`
}

type systemShutdownResponse struct {
	Accepted bool `json:"accepted"`
}

func (h *ManagementHandlers) HandleSetupStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeAuthJSON(w, http.StatusOK, setupStatusResponse{
			Initialized: h.auth != nil && h.auth.IsBootstrapped(),
		})
	}
}

func (h *ManagementHandlers) HandleSessionLogout() http.HandlerFunc {
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

func (h *ManagementHandlers) HandleLauncherStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *ManagementHandlers) HandleSystemStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.writeSystemStatus(w, http.StatusOK)
	}
}

func (h *ManagementHandlers) HandleSystemShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}

func (h *ManagementHandlers) HandleLauncherShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		h.requestShutdown()
		h.system.PublishStatusSnapshot()
		writeAuthJSON(w, http.StatusAccepted, systemShutdownResponse{Accepted: true})
	}
}

func (h *ManagementHandlers) writeSystemStatus(w http.ResponseWriter, statusCode int) {
	writeAuthJSON(w, statusCode, h.system.ManagementStatusSnapshot())
}
