package dispatch

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

// runtimeDeliverer is the interface a plugin runtime must satisfy for dispatch.
type runtimeDeliverer interface {
	DeliverEvent(context.Context, runtimeprotocol.Event) (runtimemanager.Delivery, error)
	Snapshot() runtimemanager.Snapshot
}

// Outcome represents the result of delivering an event to a single plugin.
type Outcome string

const (
	OutcomeDelivered Outcome = "delivered"
	OutcomeError     Outcome = "error"
	OutcomeDropped   Outcome = "dropped"
	OutcomeIgnored   Outcome = "ignored"
)

// DeliveryResult records the outcome of event delivery to a single plugin.
type DeliveryResult struct {
	PluginID  string
	Outcome   Outcome
	ErrorCode string
}

// CommandDecl captures a plugin's declared command for directed delivery.
type CommandDecl struct {
	Name       string
	Aliases    []string
	Permission string
}
type dispatchItem struct {
	ctx   context.Context
	event runtimeprotocol.Event
}
type pluginSlot struct {
	runtime       runtimeDeliverer
	subscriptions []string
	commands      []CommandDecl
	concurrency   int
	queue         chan dispatchItem
	done          chan struct{}
}
type CapabilityChecker func(context.Context, string, string) bool

// DispatcherStats summarises cumulative per-dispatch outcomes so consumers
// (the bridge runtime observability frame and the Prometheus metrics handler)
// can read aggregate counts without holding the dispatcher lock.
//
// Counter semantics:
//   - Delivered:    target plugins that accepted the event onto their queue
//   - Dropped:      target plugins refused due to queue full or runtime not running
//   - Errored:      runtime-level errors after delivery (worker failures, etc.)
//   - Ignored:      Dispatch() calls where no plugin matched (no target selected)
type DispatcherStats struct {
	Delivered     uint64
	Dropped       uint64
	Errored       uint64
	Ignored       uint64
	DropsByReason map[string]map[string]uint64 // reason -> plugin_id -> count
}

// MetricsObserver routes dispatcher events into the Prometheus registry
// without forcing this package to depend on client_golang. Implementations
// must be safe for concurrent use.
type MetricsObserver interface {
	IncDispatcherDrop(pluginID, reason string)
	IncEventPipelineStage(stage, outcome string)
	IncOutboundSend(adapter, outcome string)
	ObserveOutboundDuration(adapter string, duration time.Duration)
}

// Dispatcher manages per-plugin event queues and fan-out delivery.
type Dispatcher struct {
	logger            *slog.Logger
	sender            outbound.ActionSender
	resolver          outbound.ReplyTargetResolver
	outboundLimiter   outbound.MessageLimiter
	queueSize         int
	mu                sync.RWMutex
	slots             map[string]*pluginSlot
	capabilityChecker CapabilityChecker

	statsMu       sync.Mutex
	delivered     uint64
	dropped       uint64
	errored       uint64
	ignored       uint64
	dropsByReason map[string]map[string]uint64

	flushMu          sync.Mutex
	flushBaseline    DispatcherStats
	runtimePublisher DispatcherRuntimePublisher
	flushStop        chan struct{}
	flushDone        chan struct{}
	metrics          MetricsObserver
}

// New creates a Dispatcher.
func New(logger *slog.Logger, sender outbound.ActionSender, resolver outbound.ReplyTargetResolver, queueSize int) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if queueSize <= 0 {
		queueSize = 16
	}
	return &Dispatcher{
		logger:        logger,
		sender:        sender,
		resolver:      resolver,
		queueSize:     queueSize,
		slots:         make(map[string]*pluginSlot),
		dropsByReason: make(map[string]map[string]uint64),
	}
}
func (d *Dispatcher) SetCapabilityChecker(checker CapabilityChecker) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.capabilityChecker = checker
}
func (d *Dispatcher) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.outboundLimiter = limiter
}
