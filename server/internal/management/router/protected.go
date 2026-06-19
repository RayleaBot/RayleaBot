package router

import "github.com/go-chi/chi/v5"

func registerProtectedRoutes(r chi.Router, deps Deps) {
	for _, module := range deps.ProtectedRoutes {
		if module != nil {
			module.RegisterProtectedRoutes(r)
		}
	}
}
