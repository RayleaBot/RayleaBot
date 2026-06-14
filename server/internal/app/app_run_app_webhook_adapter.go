package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
)

type webhookGateway interface {
	Expose(context.Context, string, runtimeaction.Action) (map[string]any, error)
}

// webhookReplayMetricsAdapter exposes the metrics.Registry replay counter
// behind the narrow ReplayMetricsObserver interface so pluginwebhook can
// record outcomes without importing client_golang directly.
type webhookReplayMetricsAdapter struct {
	registry *metrics.Registry
}

func (a webhookReplayMetricsAdapter) IncReplayObserved(outcome string) {
	if a.registry == nil || a.registry.WebhookReplayObserved == nil {
		return
	}
	a.registry.WebhookReplayObserved.WithLabelValues(outcome).Inc()
}
