package app

import "github.com/RayleaBot/RayleaBot/server/internal/auth"

type authHTTPHandlers struct {
	config        authHTTPConfigSource
	auth          authSessionService
	loginFailures loginFailureRecorder
}

func newAuthHTTPHandlers(deps authHTTPDeps) *authHTTPHandlers {
	return &authHTTPHandlers{
		config:        deps.config,
		auth:          deps.auth,
		loginFailures: deps.loginFailures,
	}
}

type authSessionService interface {
	Bootstrap(string, string) (string, auth.Claims, error)
	Login(string, string) (string, auth.Claims, error)
}
