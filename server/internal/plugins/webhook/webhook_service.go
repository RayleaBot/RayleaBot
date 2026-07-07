package webhook

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
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

type Registry struct {
	mu    sync.RWMutex
	items map[string]Registration
}

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
	WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool)
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
	Plugins       plugins.CatalogView
	Dispatcher    *dispatch.Dispatcher
	Runtime       RuntimeEnsurer
	Capabilities  CapabilityView
}

type Service struct {
	currentConfig func() config.Config
	logger        *slog.Logger
	registry      *Registry
	secrets       secrets.Store
	plugins       plugins.CatalogView
	dispatcher    *dispatch.Dispatcher
	runtime       RuntimeEnsurer
	capabilities  CapabilityView

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
		capabilities:  deps.Capabilities,
		dedup:         newReplayCache(),
		now:           time.Now,
	}
}

// SetReplayMetrics wires a metrics observer that records every replay
// protection outcome ("rejected", "grace_observed", "skew"). Optional; the
// service runs without it when nil.
func (s *Service) SetReplayMetrics(observer ReplayMetricsObserver) {
	s.metrics = observer
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

func (r *Registry) Register(item Registration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[webhookKey(item.PluginID, item.Route)] = item
}

func (r *Registry) Get(pluginID, route string) (Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[webhookKey(pluginID, route)]
	return item, ok
}

func (r *Registry) DeletePlugin(pluginID string) {
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

// replayDecision summarises the replay-protection outcome for a single
// webhook request. When reject is false the request continues into HMAC
// validation; the parsed timestamp / event id are reused to assemble the
// downstream plugin event so the plugin sees consistent identifiers. The
// dedup key + ttl are populated when peek-then-commit is in play so the
// caller can mark the (route, event_id) as seen only after authentication
// succeeds.
type replayDecision struct {
	reject       bool
	code         string
	messageKey   string
	timestamp    int64
	timestampRaw string
	eventID      string
	dedupKey     string
	dedupTTL     time.Duration
}

// replayCache is the in-memory LRU+TTL set the webhook service uses to
// detect duplicate (route, event_id) pairs within the replay tolerance
// window. Reads, writes, and eviction all live under a single mutex; the
// expected cardinality stays in the low thousands per route.
type replayCache struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func (s *Service) evaluateReplayProtection(pluginID, route string, cfg ReplayProtection, r *http.Request) replayDecision {
	timestampRaw := strings.TrimSpace(r.Header.Get(cfg.TimestampHeader))
	eventID := strings.TrimSpace(r.Header.Get(cfg.EventIDHeader))
	decision := replayDecision{timestampRaw: timestampRaw, eventID: eventID}

	if timestampRaw == "" || eventID == "" {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_replay_rejected"
			decision.messageKey = "errors.plugin.webhook_replay_rejected"
			s.recordReplayMetric("rejected")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	timestamp, parseErr := strconv.ParseInt(timestampRaw, 10, 64)
	if parseErr != nil {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_timestamp_skew"
			decision.messageKey = "errors.plugin.webhook_timestamp_skew"
			s.recordReplayMetric("skew")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}
	decision.timestamp = timestamp

	now := s.now().Unix()
	tolerance := int64(cfg.ToleranceSeconds)
	if tolerance <= 0 {
		tolerance = 300
	}
	if now-timestamp > tolerance || timestamp-now > tolerance {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_timestamp_skew"
			decision.messageKey = "errors.plugin.webhook_timestamp_skew"
			s.recordReplayMetric("skew")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	dedupKey := webhookKey(pluginID, route) + "\x00" + eventID
	ttl := time.Duration(2*tolerance) * time.Second
	decision.dedupKey = dedupKey
	decision.dedupTTL = ttl
	if s.dedup.peek(dedupKey, s.now(), ttl) {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_replay_rejected"
			decision.messageKey = "errors.plugin.webhook_replay_rejected"
			s.recordReplayMetric("rejected")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	return decision
}

func (s *Service) recordReplayMetric(outcome string) {
	if s.metrics == nil {
		return
	}
	s.metrics.IncReplayObserved(outcome)
}

func newReplayCache() *replayCache {
	return &replayCache{items: make(map[string]time.Time)}
}

// peek reports whether the given key would be treated as a duplicate at
// observedAt without mutating the cache. Use it for the read-only check
// before authentication; the authoritative duplicate decision is made
// later by commitIfAbsent under a single critical section.
func (c *replayCache) peek(key string, observedAt time.Time, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(observedAt, ttl)

	seenAt, ok := c.items[key]
	if !ok {
		return false
	}
	return observedAt.Sub(seenAt) <= ttl
}

// commitIfAbsent atomically checks for a live duplicate and, if none is
// found, records the key as seen at observedAt. It returns true when the
// caller is the unique winner for the (key, ttl) window and may proceed,
// and false when another concurrent request already won the slot.
//
// Callers must invoke commitIfAbsent only after authentication has
// succeeded. Splitting the duplicate check into peek + commit would leave
// a race where two authenticated callers both peek empty before either
// commits; commitIfAbsent collapses both steps under one lock so replay
// protection holds even under concurrent legitimate retries.
func (c *replayCache) commitIfAbsent(key string, observedAt time.Time, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(observedAt, ttl)

	if seenAt, ok := c.items[key]; ok {
		if observedAt.Sub(seenAt) <= ttl {
			return false
		}
	}
	c.items[key] = observedAt
	return true
}

func (c *replayCache) purgeExpiredLocked(observedAt time.Time, ttl time.Duration) {
	for cached, seenAt := range c.items {
		if observedAt.Sub(seenAt) > ttl {
			delete(c.items, cached)
		}
	}
}

// Reset drops every cached entry. Intended for tests.
func (c *replayCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]time.Time)
}
