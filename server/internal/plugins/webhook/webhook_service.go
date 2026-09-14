package webhook

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

// Registration is one manifest webhook route. The host only forwards the
// request; plugins verify signatures and handle duplicate deliveries.
type Registration struct {
	PluginID     string
	Route        string
	Methods      []string
	SourceIPs    []string
	MaxBodyBytes int
}

type Registry struct {
	mu    sync.RWMutex
	items map[string]Registration
}

type RuntimeEnsurer interface {
	EnsurePluginRunning(context.Context, string) error
}

type Deps struct {
	Logger     *slog.Logger
	Registry   *Registry
	Plugins    plugins.CatalogView
	Dispatcher *dispatch.Dispatcher
	Runtime    RuntimeEnsurer
}

type Service struct {
	logger     *slog.Logger
	registry   *Registry
	plugins    plugins.CatalogView
	dispatcher *dispatch.Dispatcher
	runtime    RuntimeEnsurer
	now        func() time.Time
}

func New(deps Deps) (*Service, error) {
	if deps.Registry == nil || deps.Plugins == nil || deps.Dispatcher == nil || deps.Runtime == nil {
		return nil, errors.New("plugin webhook service requires registry, plugin catalog, dispatcher, and runtime")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger:     logger,
		registry:   deps.Registry,
		plugins:    deps.Plugins,
		dispatcher: deps.Dispatcher,
		runtime:    deps.Runtime,
		now:        time.Now,
	}, nil
}

func (s *Service) SyncManifestRegistrations() {
	s.registry.SyncSnapshots(s.plugins.List())
}

func (r *Registry) SyncSnapshots(snapshots []plugins.Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[string]Registration)
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != plugins.RegistrationStateInstalled {
			continue
		}
		for _, webhook := range snapshot.Webhooks {
			registration := Registration{
				PluginID: snapshot.PluginID, Route: webhook.Route, Methods: []string{http.MethodPost},
				SourceIPs: append([]string(nil), webhook.SourceCIDRs...), MaxBodyBytes: webhook.MaxBodyBytes,
			}
			r.items[webhookKey(registration.PluginID, registration.Route)] = registration
		}
	}
}

func (s *Service) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Post("/api/webhooks/{plugin_id}/{route}", s.HandleWebhook())
}

func NewRegistry() *Registry {
	return &Registry{
		items: make(map[string]Registration),
	}
}

func (r *Registry) Get(pluginID, route string) (Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[webhookKey(pluginID, route)]
	return item, ok
}

func webhookKey(pluginID, route string) string {
	return strings.TrimSpace(pluginID) + "\x00" + strings.TrimSpace(route)
}
