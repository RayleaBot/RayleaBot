package authapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const (
	codePermissionDenied = "permission.denied"
	codeInvalidRequest   = "platform.invalid_request"
	codeInternalError    = "platform.internal_error"
)

type Config struct {
	SetupLocalOnly     bool
	LoginFailureLimit  int
	LoginFailureWindow time.Duration
}

type ConfigSource interface {
	AuthConfig() Config
}

type Handlers struct {
	config        ConfigSource
	auth          authSessionService
	loginFailures LoginFailureRecorder
}

type Deps struct {
	Config        ConfigSource
	Auth          authSessionService
	LoginFailures LoginFailureRecorder
}

func NewHandlers(deps Deps) *Handlers {
	return &Handlers{
		config:        deps.Config,
		auth:          deps.Auth,
		loginFailures: deps.LoginFailures,
	}
}

func NewModule(deps Deps) *Handlers {
	return NewHandlers(deps)
}

func (h *Handlers) SetAuthManager(manager authSessionService) {
	if h == nil {
		return
	}
	h.auth = manager
}

type authSessionService interface {
	BootstrapWithContext(context.Context, string, string) (string, auth.Claims, error)
	LoginWithContext(context.Context, string, string) (string, auth.Claims, error)
}

func (h *Handlers) currentConfig() Config {
	if h == nil || h.config == nil {
		return Config{}
	}
	return h.config.AuthConfig()
}

type authRequest struct {
	Identifier string `json:"identifier"`
	Secret     string `json:"secret"`
}

type authResponse struct {
	SessionToken string `json:"session_token"`
}

func (h *Handlers) HandleSetupAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentConfig()
		if cfg.SetupLocalOnly && !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		token, _, err := h.auth.BootstrapWithContext(r.Context(), request.Identifier, request.Secret)
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

func (h *Handlers) HandleSessionLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentConfig()
		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		sourceIP := httpapi.RequestRemoteIP(r)
		if h.loginFailures != nil && h.loginFailures.IsLimited(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow) {
			writeAppError(w, r, http.StatusTooManyRequests, "platform.rate_limited", "触发平台级限流", "errors.platform.rate_limited", nil)
			return
		}

		token, _, err := h.auth.LoginWithContext(r.Context(), request.Identifier, request.Secret)
		switch {
		case err == nil:
			if h.loginFailures != nil {
				h.loginFailures.Reset(sourceIP)
			}
			writeAuthJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			if h.loginFailures != nil {
				h.loginFailures.RecordFailure(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow)
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

type LoginFailureRecorder = auth.LoginFailureRecorder

func LoginFailureLimit(cfg config.Config) int {
	return cfg.Admin.LoginFailLimit
}

func LoginFailureWindow(cfg config.Config) time.Duration {
	seconds := cfg.Admin.LoginFailWindowSecs
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
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
