package managementhttp

import "github.com/RayleaBot/RayleaBot/server/internal/auth"

type AuthHandlers struct {
	config        AuthConfigSource
	auth          authSessionService
	loginFailures LoginFailureRecorder
}

type AuthDeps struct {
	Config        AuthConfigSource
	Auth          authSessionService
	LoginFailures LoginFailureRecorder
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

type authSessionService interface {
	Bootstrap(string, string) (string, auth.Claims, error)
	Login(string, string) (string, auth.Claims, error)
}
