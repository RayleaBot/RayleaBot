package render

import (
	"errors"
	"time"
)

func (s *Service) SetMetricsObserver(observer MetricsObserver) {
	if s == nil {
		return
	}
	s.metricsMu.Lock()
	s.metrics = observer
	s.metricsMu.Unlock()
}

func (s *Service) currentMetrics() MetricsObserver {
	if s == nil {
		return nil
	}
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()
	return s.metrics
}

func (s *Service) recordRenderMetric(outcome string, duration time.Duration) {
	observer := s.currentMetrics()
	if observer == nil {
		return
	}
	observer.ObserveRenderDuration(outcome, duration)
}

func renderOutcome(result Result, err error) string {
	if err != nil {
		var renderErr *Error
		if errors.As(err, &renderErr) {
			switch renderErr.Code {
			case "platform.render_queue_full":
				return "queue_full"
			case "platform.render_timeout":
				return "timeout"
			}
		}
		return "failed"
	}
	if result.FromCache {
		return "cache_hit"
	}
	return "succeeded"
}
