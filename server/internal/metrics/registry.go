// Package metrics owns the platform-wide Prometheus instrumentation surface.
// It exposes a registry that wraps prometheus.Registry and pre-declares every
// metric name and label keyset the server reports, so callers never invent
// names ad hoc and label cardinality stays bounded.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

const namespace = "raylea"

// Registry aggregates all formal metric handles. Each field is non-nil after
// New and safe to use concurrently. Callers must not register additional
// collectors against the underlying prometheus.Registry; introduce new fields
// here instead.
type Registry struct {
	registry *prometheus.Registry

	EventPipelineStage   *prometheus.CounterVec
	PluginRuntimeState   *prometheus.GaugeVec
	TaskExecutionLatency *prometheus.HistogramVec
	RenderQueueDepth     prometheus.Gauge
	RenderDuration       *prometheus.HistogramVec
	OutboundSendTotal    *prometheus.CounterVec
	OutboundSendDuration *prometheus.HistogramVec
	DispatcherDropTotal  *prometheus.CounterVec
	DispatcherQueueDepth *prometheus.GaugeVec
	AdapterDedupDrops    prometheus.Counter
	BridgeIgnoredTotal   prometheus.Counter
	WebhookReplayObserved *prometheus.CounterVec
}

// New builds a Registry with every formal collector pre-registered. A nil
// Registry is never returned; failures to register a collector cause a panic
// at startup, mirroring expvar semantics.
func New() *Registry {
	reg := prometheus.NewRegistry()
	r := &Registry{registry: reg}

	r.EventPipelineStage = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "event_pipeline_stage_total",
		Help:      "Events flowing through each pipeline stage by outcome.",
	}, []string{"stage", "outcome"})

	r.PluginRuntimeState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "plugin_runtime_state",
		Help:      "Current count of plugin runtimes per state.",
	}, []string{"state"})

	r.TaskExecutionLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "task_execution_duration_seconds",
		Help:      "Background task execution duration grouped by task type and outcome.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"task_type", "outcome"})

	r.RenderQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "render_queue_depth",
		Help:      "Depth of the render service request queue.",
	})

	r.RenderDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "render_duration_seconds",
		Help:      "Render request total handling duration (queue wait plus rendering).",
		Buckets:   prometheus.DefBuckets,
	}, []string{"outcome"})

	r.OutboundSendTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "outbound_send_total",
		Help:      "Outbound send attempts grouped by adapter and outcome.",
	}, []string{"adapter", "outcome"})

	r.OutboundSendDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "outbound_send_duration_seconds",
		Help:      "Outbound send wall-clock latency grouped by adapter.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"adapter"})

	r.DispatcherDropTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "dispatcher_drop_total",
		Help:      "Dispatcher drops grouped by plugin id and drop reason.",
	}, []string{"plugin_id", "reason"})

	r.DispatcherQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "dispatcher_queue_depth",
		Help:      "Current depth of each per-plugin dispatcher queue.",
	}, []string{"plugin_id"})

	r.AdapterDedupDrops = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "adapter_dedup_drops_total",
		Help:      "Adapter dedup drops within the configured retention window.",
	})

	r.BridgeIgnoredTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "bridge_ignored_total",
		Help:      "Bridge ignored events that found no interested plugin runtime.",
	})

	r.WebhookReplayObserved = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "plugin_webhook_replay_observed_total",
		Help:      "Plugin webhook replay-protection observations grouped by outcome (rejected, grace_observed, skew).",
	}, []string{"outcome"})

	reg.MustRegister(
		r.EventPipelineStage,
		r.PluginRuntimeState,
		r.TaskExecutionLatency,
		r.RenderQueueDepth,
		r.RenderDuration,
		r.OutboundSendTotal,
		r.OutboundSendDuration,
		r.DispatcherDropTotal,
		r.DispatcherQueueDepth,
		r.AdapterDedupDrops,
		r.BridgeIgnoredTotal,
		r.WebhookReplayObserved,
	)
	return r
}

// HTTPHandler returns the Prometheus text-exposition HTTP handler bound to the
// owned registry.
func (r *Registry) HTTPHandler() http.Handler {
	if r == nil || r.registry == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "metrics registry not initialised", http.StatusInternalServerError)
		})
	}
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})
}

// PrometheusRegistry exposes the underlying registry for callers that need to
// register custom collectors (eg. test scaffolding).
func (r *Registry) PrometheusRegistry() *prometheus.Registry {
	if r == nil {
		return nil
	}
	return r.registry
}
