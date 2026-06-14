package metrics

import (
	"time"
)

type TaskObserver struct {
	registry *Registry
}

func NewTaskObserver(registry *Registry) TaskObserver {
	return TaskObserver{registry: registry}
}

func (a TaskObserver) ObserveTaskExecution(taskType, outcome string, duration time.Duration) {
	if a.registry == nil || a.registry.TaskExecutionLatency == nil {
		return
	}
	a.registry.TaskExecutionLatency.WithLabelValues(taskType, outcome).Observe(duration.Seconds())
}
