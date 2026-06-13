package pluginwebhook

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type ReplayProtection struct {
	TimestampHeader  string
	EventIDHeader    string
	ToleranceSeconds int
	Enforce          bool
}

type Registration struct {
	PluginID         string
	Route            string
	Methods          []string
	AuthStrategy     string
	Header           string
	SecretRef        string
	SignaturePrefix  string
	SourceIPs        []string
	URL              string
	ReplayProtection ReplayProtection
}

type GrantView interface {
	CapabilityGranted(context.Context, string, string) bool
	GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool)
}

type RuntimeEnsurer interface {
	CurrentBotID() string
	EnsurePluginRunning(context.Context, string, string) error
}

type Deps struct {
	CurrentConfig func() config.Config
	Logger        *slog.Logger
	Registry      *Registry
	Secrets       secrets.Store
	Plugins       *plugins.Catalog
	Dispatcher    *dispatch.Dispatcher
	Runtime       RuntimeEnsurer
	Grants        GrantView
}

type Service struct {
	currentConfig func() config.Config
	logger        *slog.Logger
	registry      *Registry
	secrets       secrets.Store
	plugins       *plugins.Catalog
	dispatcher    *dispatch.Dispatcher
	runtime       RuntimeEnsurer
	grants        GrantView

	dedup   *replayCache
	now     func() time.Time
	metrics ReplayMetricsObserver
}

// ReplayMetricsObserver is a narrow hook for the Prometheus registry; the
// pluginwebhook package keeps it interface-shaped so tests can stub it out
// without pulling in client_golang.
type ReplayMetricsObserver interface {
	IncReplayObserved(outcome string)
}

func New(deps Deps) *Service {
	return &Service{
		currentConfig: deps.CurrentConfig,
		logger:        deps.Logger,
		registry:      deps.Registry,
		secrets:       deps.Secrets,
		plugins:       deps.Plugins,
		dispatcher:    deps.Dispatcher,
		runtime:       deps.Runtime,
		grants:        deps.Grants,
		dedup:         newReplayCache(),
		now:           time.Now,
	}
}

// SetReplayMetrics wires a metrics observer that records every replay
// protection outcome ("rejected", "grace_observed", "skew"). Optional; the
// service runs without it when nil.
func (s *Service) SetReplayMetrics(observer ReplayMetricsObserver) {
	if s == nil {
		return
	}
	s.metrics = observer
}

func (s *Service) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Post("/api/webhooks/{plugin_id}/{route}", s.HandleWebhook())
}
