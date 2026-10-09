package adapters

import (
	"maps"
	"slices"
	"sync/atomic"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// Registry publishes configuration and runtime membership together. Only the
// owning Service replaces it; readers keep one snapshot for their operation.
type Registry struct {
	current atomic.Pointer[RuntimeSnapshot]
}

type RuntimeSnapshot struct {
	cfg        config.Config
	oneBot11   map[string]*onebot11.Shell
	qqOfficial map[string]QQOfficialAdapter
}

func NewRegistry(cfg config.Config, oneBot11 map[string]*onebot11.Shell, qqOfficial map[string]QQOfficialAdapter) *Registry {
	r := &Registry{}
	r.store(cfg, maps.Clone(oneBot11), maps.Clone(qqOfficial))
	return r
}

func (r *Registry) Snapshot() *RuntimeSnapshot { return r.current.Load() }

func (r *Registry) store(cfg config.Config, oneBot11 map[string]*onebot11.Shell, qqOfficial map[string]QQOfficialAdapter) {
	// Config callers may reuse their slices while preparing the next reload.
	cfg.Adapters = slices.Clone(cfg.Adapters)
	for i := range cfg.Adapters {
		instance := &cfg.Adapters[i]
		if instance.OneBot11 != nil {
			settings := *instance.OneBot11
			instance.OneBot11 = &settings
		}
		if instance.QQOfficial != nil {
			settings := *instance.QQOfficial
			settings.Intents = slices.Clone(settings.Intents)
			instance.QQOfficial = &settings
		}
	}
	r.current.Store(&RuntimeSnapshot{cfg: cfg, oneBot11: oneBot11, qqOfficial: qqOfficial})
}

// Config is read-only, as are the configuration snapshots published by App.
func (s *RuntimeSnapshot) Config() config.Config                  { return s.cfg }
func (s *RuntimeSnapshot) OneBot11(id string) *onebot11.Shell     { return s.oneBot11[id] }
func (s *RuntimeSnapshot) QQOfficial(id string) QQOfficialAdapter { return s.qqOfficial[id] }

func (r *Registry) DedupDropsSnapshot() uint64 {
	var total uint64
	for _, shell := range r.Snapshot().oneBot11 {
		total += shell.DedupDropsSnapshot()
	}
	return total
}
