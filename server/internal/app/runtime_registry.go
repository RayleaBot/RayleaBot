package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"rayleabot/server/internal/runtime"
)

type runtimeRegistry struct {
	logger  *slog.Logger
	options runtime.Options

	mu      sync.RWMutex
	onCrash runtime.CrashCallback
	items   map[string]*runtime.Manager
}

func newRuntimeRegistry(logger *slog.Logger, options runtime.Options) *runtimeRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeRegistry{
		logger:  logger,
		options: options,
		items:   make(map[string]*runtime.Manager),
	}
}

func (r *runtimeRegistry) SetOnCrash(callback runtime.CrashCallback) {
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

func (r *runtimeRegistry) Get(pluginID string) (*runtime.Manager, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	manager, ok := r.items[pluginID]
	return manager, ok
}

func (r *runtimeRegistry) GetOrCreate(pluginID string) *runtime.Manager {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if manager, ok := r.items[pluginID]; ok {
		return manager
	}

	manager := runtime.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	r.items[pluginID] = manager
	return manager
}

func (r *runtimeRegistry) NewDetached() *runtime.Manager {
	if r == nil {
		return nil
	}

	manager := runtime.New(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	return manager
}

func (r *runtimeRegistry) Replace(pluginID string, manager *runtime.Manager) *runtime.Manager {
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

func (r *runtimeRegistry) Delete(pluginID string) *runtime.Manager {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	manager := r.items[pluginID]
	delete(r.items, pluginID)
	return manager
}

func (r *runtimeRegistry) ActiveCount() int {
	if r == nil {
		return 0
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	for _, manager := range r.items {
		switch manager.Snapshot().State {
		case runtime.StateStarting, runtime.StateRunning, runtime.StateStopping:
			active++
		}
	}
	return active
}

func (r *runtimeRegistry) StopAll(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	managers := make([]*runtime.Manager, 0, len(r.items))
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
