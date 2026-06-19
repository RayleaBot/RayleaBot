package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	managementui "github.com/RayleaBot/RayleaBot/server/internal/management/ui"
)

type PublicRouteModule interface {
	RegisterPublicRoutes(chi.Router)
}

type ProtectedRouteModule interface {
	RegisterProtectedRoutes(chi.Router)
}

type PublicRouteFunc func(chi.Router)

func (fn PublicRouteFunc) RegisterPublicRoutes(r chi.Router) {
	if fn != nil {
		fn(r)
	}
}

type ProtectedRouteFunc func(chi.Router)

func (fn ProtectedRouteFunc) RegisterProtectedRoutes(r chi.Router) {
	if fn != nil {
		fn(r)
	}
}

type Deps struct {
	RepoRoot        string
	Readiness       func() health.ReadinessReport
	PublicRoutes    []PublicRouteModule
	ProtectedRoutes []ProtectedRouteModule
}

func Register(r chi.Router, deps Deps, requireAuth func(http.Handler) http.Handler) {
	registerPublicRoutes(r, deps)
	r.Group(func(protected chi.Router) {
		protected.Use(requireAuth)
		registerProtectedRoutes(protected, deps)
	})
	r.NotFound(managementui.NewManagementUIHandler(deps.RepoRoot))
}
