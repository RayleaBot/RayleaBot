package runtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

type Registry struct {
	logger  *slog.Logger
	options Options

	mu      sync.RWMutex
	onCrash CrashCallback
	items   map[string]*Manager
}

func NewRegistry(logger *slog.Logger, options Options) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		logger:  logger,
		options: options,
		items:   make(map[string]*Manager),
	}
}

func NewManaged(
	logger *slog.Logger,
	consoleStream *console.Stream,
	redactText func(string) string,
	stderrRateLimitBytesPerSec int,
	executeLocalAction LocalActionExecutor,
) *Registry {
	return NewRegistry(logger, Options{
		Console:                    consoleStream,
		RedactText:                 redactText,
		StderrRateLimitBytesPerSec: stderrRateLimitBytesPerSec,
		ExecuteLocalAction:         executeLocalAction,
	})
}

func (r *Registry) SetOnCrash(callback CrashCallback) {

	r.mu.Lock()
	defer r.mu.Unlock()

	r.onCrash = callback
	for _, manager := range r.items {
		manager.SetOnCrash(callback)
	}
}

func (r *Registry) Get(pluginID string) (*Manager, bool) {

	r.mu.RLock()
	defer r.mu.RUnlock()

	manager, ok := r.items[pluginID]
	return manager, ok
}

func (r *Registry) GetOrCreate(pluginID string) *Manager {

	r.mu.Lock()
	defer r.mu.Unlock()

	if manager, ok := r.items[pluginID]; ok {
		return manager
	}

	manager := NewManager(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	r.items[pluginID] = manager
	return manager
}

func (r *Registry) NewDetached() *Manager {

	manager := NewManager(r.logger, r.options)
	manager.SetOnCrash(r.onCrash)
	return manager
}

func (r *Registry) Replace(pluginID string, manager *Manager) *Manager {
	if manager == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	manager.SetOnCrash(r.onCrash)
	previous := r.items[pluginID]
	r.items[pluginID] = manager
	return previous
}

func (r *Registry) Delete(pluginID string) *Manager {

	r.mu.Lock()
	defer r.mu.Unlock()

	manager := r.items[pluginID]
	delete(r.items, pluginID)
	return manager
}

func (r *Registry) ActiveCount() int {

	r.mu.RLock()
	defer r.mu.RUnlock()

	active := 0
	for _, manager := range r.items {
		switch manager.Snapshot().State {
		case StateStarting, StateRunning, StateStopping:
			active++
		}
	}
	return active
}

func (r *Registry) StopAll(ctx context.Context) error {

	r.mu.RLock()
	managers := make([]*Manager, 0, len(r.items))
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
