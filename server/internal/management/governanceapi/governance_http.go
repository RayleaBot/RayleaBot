package governanceapi

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
)

type Handlers struct {
	service *governance.Service
}

type ModuleDeps struct {
	Service *governance.Service
}

func NewHandlers(deps governance.Deps) *Handlers {
	return NewHandlersWithService(governance.NewService(deps))
}

func NewHandlersWithService(service *governance.Service) *Handlers {
	return &Handlers{service: service}
}

func NewModule(deps ModuleDeps) *Handlers {
	return NewHandlersWithService(deps.Service)
}

func (h *Handlers) RegisterProtectedRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Get("/api/governance/blacklist", h.handleGovernanceBlacklist())
	router.Post("/api/governance/blacklist/entries", h.handleGovernanceBlacklistEntryUpsert())
	router.Delete("/api/governance/blacklist/entries/{entry_type}/{target_id}", h.handleGovernanceBlacklistEntryDelete())
	router.Get("/api/governance/whitelist", h.handleGovernanceWhitelist())
	router.Put("/api/governance/whitelist/state", h.handleGovernanceWhitelistStatePut())
	router.Post("/api/governance/whitelist/entries", h.handleGovernanceWhitelistEntryUpsert())
	router.Delete("/api/governance/whitelist/entries/{entry_type}/{target_id}", h.handleGovernanceWhitelistEntryDelete())
	router.Get("/api/governance/command-policy", h.handleGovernanceCommandPolicy())
}
