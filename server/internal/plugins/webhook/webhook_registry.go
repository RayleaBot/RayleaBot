package webhook

import (
	"strings"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	items map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{
		items: make(map[string]Registration),
	}
}

func (r *Registry) Register(item Registration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[webhookKey(item.PluginID, item.Route)] = item
}

func (r *Registry) Get(pluginID, route string) (Registration, bool) {
	if r == nil {
		return Registration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[webhookKey(pluginID, route)]
	return item, ok
}

func (r *Registry) DeletePlugin(pluginID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := pluginID + "\x00"
	for key := range r.items {
		if strings.HasPrefix(key, prefix) {
			delete(r.items, key)
		}
	}
}

func webhookKey(pluginID, route string) string {
	return strings.TrimSpace(pluginID) + "\x00" + strings.TrimSpace(route)
}
