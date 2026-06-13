package tasks

import "time"

func (e *Executor) SetMetricsObserver(observer MetricsObserver) {
	if e == nil {
		return
	}
	e.metricsMu.Lock()
	defer e.metricsMu.Unlock()
	e.metrics = observer
}

func (e *Executor) currentMetrics() MetricsObserver {
	if e == nil {
		return nil
	}
	e.metricsMu.RLock()
	defer e.metricsMu.RUnlock()
	return e.metrics
}

func (e *Executor) recordTaskMetric(taskType, outcome string, duration time.Duration) {
	observer := e.currentMetrics()
	if observer == nil {
		return
	}
	observer.ObserveTaskExecution(taskType, outcome, duration)
}
