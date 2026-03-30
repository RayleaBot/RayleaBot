package app

import (
	"errors"
	"net/http"

	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/httpapi"
)

const (
	codePermissionDenied   = "permission.denied"
	codeInvalidRequest     = "platform.invalid_request"
	codeResourceMissing    = "platform.resource_missing"
	codeInternalError      = "platform.internal_error"
	codeTaskNotCancellable = "platform.task_not_cancellable"
)

type authRequest struct {
	Identifier string `json:"identifier"`
	Secret     string `json:"secret"`
}

type launcherAdmissionRequest struct {
	LauncherToken string `json:"launcher_token"`
}

type authResponse struct {
	SessionToken string `json:"session_token"`
}

func (a *App) handleSetupAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.Config.Web.SetupLocalOnly && !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		var request authRequest
		if err := decodeStrictJSON(w, r, &request, maxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		token, _, err := a.Auth.Bootstrap(request.Identifier, request.Secret)
		switch {
		case err == nil:
			writeAuthJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrBootstrapAlreadyInitialized), errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
	}
}

func (a *App) handleSessionLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request authRequest
		if err := decodeStrictJSON(w, r, &request, maxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		sourceIP := requestRemoteIP(r)
		if a.loginFailures != nil && a.loginFailures.IsLimited(sourceIP, loginFailureLimit(a.Config), loginFailureWindow(a.Config)) {
			writeAppError(w, r, http.StatusTooManyRequests, "platform.rate_limited", "触发平台级限流", "errors.platform.rate_limited", nil)
			return
		}

		token, _, err := a.Auth.Login(request.Identifier, request.Secret)
		switch {
		case err == nil:
			if a.loginFailures != nil {
				a.loginFailures.Reset(sourceIP)
			}
			writeAuthJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			if a.loginFailures != nil {
				a.loginFailures.RecordFailure(sourceIP, loginFailureLimit(a.Config), loginFailureWindow(a.Config))
			}
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		case errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
	}
}

func (a *App) handleLauncherAdmission() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if a.Auth == nil || !a.Auth.IsBootstrapped() {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		var request launcherAdmissionRequest
		if err := decodeStrictJSON(w, r, &request, maxManagementJSONBodyBytes); err != nil || request.LauncherToken == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		if !a.launcherTokens.Consume(request.LauncherToken) {
			writeAuthError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		token, _, err := a.Auth.Issue("launcher")
		switch {
		case err == nil:
			writeAuthJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
	}
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
