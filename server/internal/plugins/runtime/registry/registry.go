package registry

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Registry struct {
	logger  *slog.Logger
	options runtimemanager.Options

	mu      sync.RWMutex
	onCrash runtimemanager.CrashCallback
	items   map[string]*runtimemanager.Manager
}

func New(logger *slog.Logger, options runtimemanager.Options) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		logger:  logger,
		options: options,
		items:   make(map[string]*runtimemanager.Manager),
	}
}

func (r *Registry) SetOnCrash(callback runtimemanager.CrashCallback) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.onCrash = callback
	for _, manager := range r.items {
		manager.SetOnCrash(callback)
	}
}

func (r *Registry) Get(pluginID string) (*runtimemanager.Manager, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	manager, ok := r.items[pluginID]
	return manager, ok
}

func (r *Registry) GetOrCreate(pluginID string) *runtimemanager.Manager {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if manager, ok := r.items[pluginID]; ok {
		return manager
	}

	manager := runtimemanager.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	r.items[pluginID] = manager
	return manager
}

func (r *Registry) NewDetached() *runtimemanager.Manager {
	if r == nil {
		return nil
	}

	manager := runtimemanager.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	return manager
}

func (r *Registry) Replace(pluginID string, manager *runtimemanager.Manager) *runtimemanager.Manager {
	if r == nil || manager == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	manager.SetOnCrash(r.onCrash)
	previous := r.items[pluginID]
	r.items[pluginID] = manager
	return previous
}

func (r *Registry) Delete(pluginID string) *runtimemanager.Manager {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	manager := r.items[pluginID]
	delete(r.items, pluginID)
	return manager
}

func (r *Registry) ActiveCount() int {
	if r == nil {
		return 0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	for _, manager := range r.items {
		switch manager.Snapshot().State {
		case runtimemanager.StateStarting, runtimemanager.StateRunning, runtimemanager.StateStopping:
			active++
		}
	}
	return active
}

func (r *Registry) StopAll(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	managers := make([]*runtimemanager.Manager, 0, len(r.items))
	for _, manager := range r.items {
		managers = append(managers, manager)
	}
	r.mu.RUnlock()

	var stopErr error
	for _, manager := range managers {
		if err := manager.Stop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			stopErr = errors.Join(stopErr, err)
		}
	}
	return stopErr
}
