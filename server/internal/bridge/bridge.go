package bridge

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

const (
	codePlatformInvalidRequest   = "platform.invalid_request"
	codePluginInternalError      = "plugin.internal_error"
	eventsChannel                = "events"
	eventsTypeReceived           = "events.received"
	observabilityScopeBridge     = "bridge_runtime"
	observabilityScopeDispatcher = "dispatcher_runtime"
	summaryBridgeRuntime         = "bridge delivered recent adapter events while keeping bridge/runtime observability aggregate-only"
)

type Outcome string

const (
	OutcomeIgnored   Outcome = "ignored"
	OutcomeDelivered Outcome = "delivered"
	OutcomeError     Outcome = "error"
	OutcomeRejected  Outcome = "rejected"
)

type Snapshot struct {
	AcceptedCount  uint64
	DeliveredCount uint64
	ResultCount    uint64
	ErrorCount     uint64
	IgnoredCount   uint64
	RejectedCount  uint64
	LastEventType  string
	LastEventKind  string
	LastOutcome    Outcome
	LastErrorCode  string
	LastErrorText  string
	LastEventAt    *time.Time
}

type ObservabilityFrame struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      any    `json:"data"`
}

// DispatcherRuntimeDropRow mirrors the WebSocket-facing dispatcher_runtime
// drops_by_reason row. Plugin id and event type are optional.
type DispatcherRuntimeDropRow struct {
	Reason    string `json:"reason"`
	PluginID  string `json:"plugin_id,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Count     uint64 `json:"count"`
}

// DispatcherRuntimeData is the dispatcher_runtime branch payload pushed
// through events.received subscribers. It carries window-local deltas.
type DispatcherRuntimeData struct {
	ObservabilityScope string                     `json:"observability_scope"`
	WindowSeconds      int                        `json:"window_seconds"`
	DeliveredCount     uint64                     `json:"delivered_count"`
	DroppedCount       uint64                     `json:"dropped_count"`
	IgnoredCount       uint64                     `json:"ignored_count"`
	DropsByReason      []DispatcherRuntimeDropRow `json:"drops_by_reason,omitempty"`
}

type ObservabilityData struct {
	ObservabilityScope     string  `json:"observability_scope"`
	Summary                string  `json:"summary"`
	LastSupportedKind      string  `json:"last_supported_event_kind,omitempty"`
	LastDeliveryOutcome    Outcome `json:"last_delivery_outcome,omitempty"`
	DeliveredCount         uint64  `json:"delivered_count"`
	ResultCount            uint64  `json:"result_count"`
	ErrorCount             uint64  `json:"error_count"`
	AdapterDedupDropsTotal uint64  `json:"adapter_dedup_drops_total,omitempty"`
	BridgeIgnoredTotal     uint64  `json:"bridge_ignored_total,omitempty"`
	DispatcherDelivered    uint64  `json:"dispatcher_delivered_total,omitempty"`
	DispatcherDropped      uint64  `json:"dispatcher_dropped_total,omitempty"`
	DispatcherIgnored      uint64  `json:"dispatcher_ignored_total,omitempty"`
}

type dispatcherClient interface {
	HasDeliverablePlugins() bool
	Dispatch(context.Context, runtimeprotocol.Event, string) []dispatch.DeliveryResult
}

type CommandPolicyRejection struct {
	CommandName      string
	PluginID         string
	MatchedPluginIDs []string
	ErrorCode        string
	Reason           string
	ReasonSummary    string
	PolicyStage      string
}

// AdapterDedupStats reports the cumulative count of inbound events the
// adapter dropped as duplicates within the dedup retention window.
type AdapterDedupStats interface {
	DedupDropsSnapshot() uint64
}

// DispatcherStatsSnapshot reports cumulative dispatcher outcomes for
// cross-layer observability. The bridge keeps the dispatcher dependency
// loose to avoid an import cycle through internal/dispatch.
type DispatcherStatsSnapshot interface {
	Stats() DispatcherStatsView
}

type DispatcherStatsView struct {
	Delivered uint64
	Dropped   uint64
	Errored   uint64
	Ignored   uint64
}

// MetricsObserver lets the bridge increment Prometheus counters without
// importing client_golang directly. Implementations must be safe for
// concurrent use.
type MetricsObserver interface {
	IncEventPipelineStage(stage, outcome string)
	IncBridgeIgnored()
}

type Bridge struct {
	logger     *slog.Logger
	dispatcher dispatcherClient

	mu               sync.RWMutex
	snapshot         Snapshot
	nextSubscriberID uint64
	subscribers      map[uint64]chan ObservabilityFrame

	adapterStats    AdapterDedupStats
	dispatcherStats DispatcherStatsSnapshot
	metrics         MetricsObserver
}

func New(logger *slog.Logger, dispatcher dispatcherClient) *Bridge {
	if logger == nil {
		logger = slog.Default()
	}

	return &Bridge{
		logger:      logger,
		dispatcher:  dispatcher,
		subscribers: make(map[uint64]chan ObservabilityFrame),
	}
}

func (b *Bridge) Snapshot() Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	cloned := b.snapshot
	if b.snapshot.LastEventAt != nil {
		lastEventAt := *b.snapshot.LastEventAt
		cloned.LastEventAt = &lastEventAt
	}
	return cloned
}
