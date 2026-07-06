package management

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

type PublicRouteModule interface {
	RegisterPublicRoutes(chi.Router)
}

type ProtectedRouteModule interface {
	RegisterProtectedRoutes(chi.Router)
}

type Module interface {
	PublicRouteModule
	ProtectedRouteModule
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

type RouteDeps struct {
	RepoRoot        string
	Readiness       func() health.ReadinessReport
	PublicRoutes    []PublicRouteModule
	ProtectedRoutes []ProtectedRouteModule
}

func RegisterRoutes(r chi.Router, deps RouteDeps, requireAuth func(http.Handler) http.Handler) {
	registerPublicRoutes(r, deps)
	r.Group(func(protected chi.Router) {
		protected.Use(requireAuth)
		registerProtectedRoutes(protected, deps)
	})
	r.NotFound(newManagementUIHandler(deps.RepoRoot))
}

func registerPublicRoutes(r chi.Router, deps RouteDeps) {
	r.Get("/healthz", health.NewLivenessHandler())
	r.Get("/readyz", health.NewReadinessHandler(deps.Readiness))
	for _, module := range deps.PublicRoutes {
		if module != nil {
			module.RegisterPublicRoutes(r)
		}
	}
}

func registerProtectedRoutes(r chi.Router, deps RouteDeps) {
	for _, module := range deps.ProtectedRoutes {
		if module != nil {
			module.RegisterProtectedRoutes(r)
		}
	}
}
