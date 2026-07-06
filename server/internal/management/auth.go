package management

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const (
	authCodePermissionDenied = "permission.denied"
	authCodeInvalidRequest   = "platform.invalid_request"
	authCodeInternalError    = "platform.internal_error"
)

type AuthConfig struct {
	SetupLocalOnly     bool
	LoginFailureLimit  int
	LoginFailureWindow time.Duration
}

type AuthConfigSource interface {
	AuthConfig() AuthConfig
}

type AuthHandlers struct {
	config        AuthConfigSource
	auth          authSessionService
	loginFailures auth.LoginFailureRecorder
}

type AuthDeps struct {
	Config        AuthConfigSource
	Auth          authSessionService
	LoginFailures auth.LoginFailureRecorder
}

func NewAuthHandlers(deps AuthDeps) *AuthHandlers {
	return &AuthHandlers{
		config:        deps.Config,
		auth:          deps.Auth,
		loginFailures: deps.LoginFailures,
	}
}

func (h *AuthHandlers) SetAuthManager(manager authSessionService) {
	if h == nil {
		return
	}
	h.auth = manager
}

func (h *AuthHandlers) RegisterPublicRoutes(router chi.Router) {
	router.Post("/api/setup/admin", h.HandleSetupAdmin())
	router.Post("/api/session/login", h.HandleSessionLogin())
}

type authSessionService interface {
	BootstrapWithContext(context.Context, string, string) (string, auth.Claims, error)
	LoginWithContext(context.Context, string, string) (string, auth.Claims, error)
}

func (h *AuthHandlers) currentConfig() AuthConfig {
	if h == nil || h.config == nil {
		return AuthConfig{}
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

func (h *AuthHandlers) HandleSetupAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentConfig()
		if cfg.SetupLocalOnly && !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		token, _, err := h.auth.BootstrapWithContext(r.Context(), request.Identifier, request.Secret)
		switch {
		case err == nil:
			httpapi.WriteJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrBootstrapAlreadyInitialized), errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, authCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
	}
}

func (h *AuthHandlers) HandleSessionLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentConfig()
		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		sourceIP := httpapi.RequestRemoteIP(r)
		if h.loginFailures != nil && h.loginFailures.IsLimited(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "platform.rate_limited", "触发平台级限流", "errors.platform.rate_limited", nil)
			return
		}

		token, _, err := h.auth.LoginWithContext(r.Context(), request.Identifier, request.Secret)
		switch {
		case err == nil:
			if h.loginFailures != nil {
				h.loginFailures.Reset(sourceIP)
			}
			httpapi.WriteJSON(w, http.StatusOK, authResponse{SessionToken: token})
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			if h.loginFailures != nil {
				h.loginFailures.RecordFailure(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow)
			}
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		case errors.Is(err, auth.ErrSessionLimitReached):
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		default:
			httpapi.WriteError(w, r, http.StatusInternalServerError, authCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
	}
}

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
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, nil)
}

// claimsKey is an unexported type used as the context key for storing auth.Claims,
// preventing external packages from accidentally overwriting the value.
type claimsKey struct{}

// RequireAuth returns a chi-compatible middleware that validates a Bearer token
// from the Authorization header and stores the resulting Claims in the request context.
// For management WebSocket paths, it additionally supports the session_token query parameter
// as a fallback token source (Authorization header takes priority).
func RequireAuth(authManager *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)

			if token == "" && strings.HasPrefix(r.URL.Path, "/ws/") {
				token = strings.TrimSpace(r.URL.Query().Get("session_token"))
			}

			if strings.TrimSpace(token) == "" {
				writePermissionDenied(w, r)
				return
			}

			claims, err := authManager.ValidateWithContext(r.Context(), token)
			if err != nil {
				writePermissionDenied(w, r)
				return
			}

			ctx := ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ContextWithClaims(ctx context.Context, claims auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsFromContext extracts auth.Claims from the request context.
// If the context does not contain Claims (e.g. unauthenticated request),
// it returns a zero-value Claims and false.
func ClaimsFromContext(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(auth.Claims)
	return claims, ok
}

// extractBearerToken extracts the token from an "Authorization: Bearer <token>" header.
// Returns an empty string if the header is missing or does not start with "Bearer ".
func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return header[len(prefix):]
}

func writePermissionDenied(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusUnauthorized,
		authCodePermissionDenied,
		"当前用户无权执行该操作",
		"errors.permission.denied",
		nil,
	)
}
