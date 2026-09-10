package management

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
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

type RouteDeps struct {
	RepoRoot        string
	Readiness       func() systemsvc.ReadinessReport
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
	r.Get("/healthz", NewLivenessHandler())
	r.Get("/readyz", NewReadinessHandler(deps.Readiness))
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
