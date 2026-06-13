package governance

import (
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	service *Service
}

func NewHandlers(deps Deps) *Handlers {
	return NewHandlersWithService(NewService(deps))
}

func NewHandlersWithService(service *Service) *Handlers {
	return &Handlers{service: service}
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
