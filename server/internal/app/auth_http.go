package app

import (
	"encoding/json"
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

type authResponse struct {
	SessionToken string `json:"session_token"`
}

func (a *App) handleSetupAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request authRequest
		if err := decodeStrictJSON(r, &request); err != nil || request.Identifier == "" || request.Secret == "" {
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
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
	}
}

func (a *App) handleSessionLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request authRequest
		if err := decodeStrictJSON(r, &request); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		token, _, err := a.Auth.Login(request.Identifier, request.Secret)
		switch {
		case err == nil:
			writeAuthJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
	}
}

func decodeStrictJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		return nil
	}

	return errors.New("unexpected trailing JSON content")
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
