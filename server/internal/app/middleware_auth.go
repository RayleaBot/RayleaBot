package app

import (
	"context"
	"net/http"
	"strings"

	"rayleabot/server/internal/auth"
)

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
				writeAuthError(w, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
				return
			}

			claims, err := authManager.Validate(token)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
