package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
)

func registerPublicRoutes(r chi.Router, deps Deps) {
	r.Get("/healthz", health.NewLivenessHandler())
	r.Get("/readyz", health.NewReadinessHandler(deps.Readiness))
	for _, module := range deps.PublicRoutes {
		if module != nil {
			module.RegisterPublicRoutes(r)
		}
	}
}
