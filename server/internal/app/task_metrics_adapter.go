package app

import (
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
)

// taskMetricsAdapter routes task executor outcomes into the Prometheus registry.
type taskMetricsAdapter struct {
	registry *metrics.Registry
}

func (a taskMetricsAdapter) ObserveTaskExecution(taskType, outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.TaskExecutionLatency == nil {
		return
	}
	a.registry.TaskExecutionLatency.WithLabelValues(taskType, outcome).Observe(duration.Seconds())
}
