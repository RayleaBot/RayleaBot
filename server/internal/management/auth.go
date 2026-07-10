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
	AllowedHosts       []string
	AllowedOrigins     []string
	SecureCookie       bool
}

type AuthConfigSource interface {
	AuthConfig() AuthConfig
}

type AuthHandlers struct {
	config        AuthConfigSource
	auth          authSessionService
	loginFailures auth.LoginFailureRecorder
	setupToken    *OneTimeToken
}

type AuthDeps struct {
	Config        AuthConfigSource
	Auth          authSessionService
	LoginFailures auth.LoginFailureRecorder
	SetupToken    *OneTimeToken
}

func NewAuthHandlers(deps AuthDeps) *AuthHandlers {
	return &AuthHandlers{
		config:        deps.Config,
		auth:          deps.Auth,
		loginFailures: deps.LoginFailures,
		setupToken:    deps.SetupToken,
	}
}

func (h *AuthHandlers) RegisterPublicRoutes(router chi.Router) {
	router.Post("/api/setup/admin", h.HandleSetupAdmin())
	router.Post("/api/session/login", h.HandleSessionLogin())
}

type authSessionService interface {
	BootstrapWithContext(context.Context, string, string) (string, auth.Claims, error)
	LoginWithContext(context.Context, string, string) (string, auth.Claims, error)
	CSRFToken(auth.Claims) string
}

func (h *AuthHandlers) currentConfig() AuthConfig {
	if h.config == nil {
		return AuthConfig{}
	}
	return h.config.AuthConfig()
}

type authRequest struct {
	Identifier string `json:"identifier"`
	Secret     string `json:"secret"`
}

type authResponse struct {
	Transport    string `json:"transport"`
	SessionToken string `json:"session_token,omitempty"`
	CSRFToken    string `json:"csrf_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

func (h *AuthHandlers) HandleSetupAdmin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.currentConfig()
		if !validSetupRequest(r, cfg) {
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		if cfg.SetupLocalOnly && !isLoopbackRequest(r) {
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		transport, ok := sessionTransport(r)
		if !ok {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		if h.setupToken == nil || !h.setupToken.Consume(strings.TrimSpace(r.Header.Get(SetupTokenHeader))) {
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}

		token, claims, err := h.auth.BootstrapWithContext(r.Context(), request.Identifier, request.Secret)
		switch {
		case err == nil:
			h.writeSessionResponse(w, token, claims, transport, cfg)
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
		if !validRequestHost(r, cfg.AllowedHosts) || !validRequestOrigin(r, cfg.AllowedOrigins, false) {
			writeAuthError(w, r, http.StatusForbidden, authCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		var request authRequest
		if err := httpapi.DecodeStrictJSON(w, r, &request, httpapi.MaxManagementJSONBodyBytes); err != nil || request.Identifier == "" || request.Secret == "" {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}
		transport, ok := sessionTransport(r)
		if !ok {
			writeAuthError(w, r, http.StatusBadRequest, authCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request")
			return
		}

		sourceIP := httpapi.RequestRemoteIP(r)
		if h.loginFailures != nil && !h.loginFailures.Reserve(sourceIP, cfg.LoginFailureLimit, cfg.LoginFailureWindow) {
			httpapi.WriteError(w, r, http.StatusTooManyRequests, "platform.rate_limited", "触发平台级限流", "errors.platform.rate_limited", nil)
			return
		}

		token, claims, err := h.auth.LoginWithContext(r.Context(), request.Identifier, request.Secret)
		switch {
		case err == nil:
			if h.loginFailures != nil {
				h.loginFailures.Reset(sourceIP)
			}
			h.writeSessionResponse(w, token, claims, transport, cfg)
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
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

func (h *AuthHandlers) writeSessionResponse(w http.ResponseWriter, token string, claims auth.Claims, transport string, cfg AuthConfig) {
	response := authResponse{
		Transport: transport,
		ExpiresAt: claims.ExpiresAt.UTC().Format(time.RFC3339),
	}
	w.Header().Set(SessionTransportHeader, transport)
	if transport == "cookie" {
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.SecureCookie,
			SameSite: http.SameSiteStrictMode,
			Expires:  claims.ExpiresAt.UTC(),
			MaxAge:   max(1, int(time.Until(claims.ExpiresAt).Seconds())),
		})
		response.CSRFToken = h.auth.CSRFToken(claims)
	} else {
		response.SessionToken = token
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func sessionTransport(r *http.Request) (string, bool) {
	transport := strings.ToLower(strings.TrimSpace(r.Header.Get(SessionTransportHeader)))
	if transport == "" {
		return "bearer", true
	}
	return transport, transport == "bearer" || transport == "cookie"
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

// RequireAuth preserves the Bearer-only middleware entry point for non-browser
// callers and tests. Production management routes use RequireAuthWithConfig.
func RequireAuth(authManager *auth.Manager) func(http.Handler) http.Handler {
	return RequireAuthWithConfig(authManager, nil)
}

// RequireAuthWithConfig accepts Bearer authentication for API clients and the
// host-only session cookie for browsers. Unsafe cookie-authenticated requests
// additionally require the per-session CSRF value.
func RequireAuthWithConfig(authManager *auth.Manager, source AuthConfigSource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg := AuthConfig{}
			if source != nil {
				cfg = source.AuthConfig()
			}
			if !validRequestHost(r, cfg.AllowedHosts) {
				writePermissionDenied(w, r)
				return
			}

			isWebSocket := strings.HasPrefix(r.URL.Path, "/ws/")
			if isWebSocket {
				if strings.TrimSpace(r.URL.Query().Get("session_token")) != "" ||
					!validRequestOrigin(r, cfg.AllowedOrigins, true) {
					writePermissionDenied(w, r)
					return
				}
				authority, ok := normalizedOriginAuthority(r.Header.Get("Origin"))
				if !ok {
					writePermissionDenied(w, r)
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), webSocketOriginAuthorityKey{}, authority))
			}

			token := extractBearerToken(r)
			cookieAuthenticated := false
			if token == "" {
				cookie, err := r.Cookie(SessionCookieName)
				if err == nil {
					token = strings.TrimSpace(cookie.Value)
					cookieAuthenticated = token != ""
				}
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
			if cookieAuthenticated {
				w.Header().Set(CSRFHeader, authManager.CSRFToken(claims))
				if isStateChangingMethod(r.Method) {
					if !validRequestOrigin(r, cfg.AllowedOrigins, true) || !authManager.ValidateCSRF(claims, r.Header.Get(CSRFHeader)) {
						writePermissionDenied(w, r)
						return
					}
				}
				http.SetCookie(w, &http.Cookie{
					Name:     SessionCookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					Secure:   cfg.SecureCookie,
					SameSite: http.SameSiteStrictMode,
					Expires:  claims.ExpiresAt.UTC(),
					MaxAge:   max(1, int(time.Until(claims.ExpiresAt).Seconds())),
				})
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
