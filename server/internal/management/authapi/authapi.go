package authapi

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
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

func (h *Handlers) SetAuthManager(manager authSessionService) {
	if h == nil {
		return
	}
	h.auth = manager
}

type authSessionService interface {
	Bootstrap(string, string) (string, auth.Claims, error)
	Login(string, string) (string, auth.Claims, error)
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

		token, _, err := h.auth.Bootstrap(request.Identifier, request.Secret)
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

		token, _, err := h.auth.Login(request.Identifier, request.Secret)
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

type LoginFailureTracker struct {
	now func() time.Time

	mu      sync.Mutex
	entries map[string][]time.Time
}

type LoginFailureRecorder interface {
	IsLimited(string, int, time.Duration) bool
	RecordFailure(string, int, time.Duration)
	Reset(string)
}

func NewLoginFailureTracker(now func() time.Time) *LoginFailureTracker {
	if now == nil {
		now = time.Now
	}
	return &LoginFailureTracker{
		now:     now,
		entries: make(map[string][]time.Time),
	}
}

func (t *LoginFailureTracker) IsLimited(source string, limit int, window time.Duration) bool {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := t.prunedLocked(source, window)
	return len(entries) >= limit
}

func (t *LoginFailureTracker) RecordFailure(source string, limit int, window time.Duration) {
	if !loginFailureTrackingEnabled(source, limit, window) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entries := append(t.prunedLocked(source, window), t.now().UTC())
	t.entries[source] = entries
}

func (t *LoginFailureTracker) Reset(source string) {
	if t == nil || source == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, source)
}

func (t *LoginFailureTracker) prunedLocked(source string, window time.Duration) []time.Time {
	if t == nil || source == "" {
		return nil
	}

	entries := t.entries[source]
	if len(entries) == 0 {
		delete(t.entries, source)
		return nil
	}

	cutoff := t.now().UTC().Add(-window)
	filtered := entries[:0]
	for _, entry := range entries {
		if !entry.Before(cutoff) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(t.entries, source)
		return nil
	}

	t.entries[source] = filtered
	return filtered
}

func loginFailureTrackingEnabled(source string, limit int, window time.Duration) bool {
	return source != "" && limit > 0 && window > 0
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
