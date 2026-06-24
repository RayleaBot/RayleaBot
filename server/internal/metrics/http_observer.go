package metrics

import (
	"strconv"
	"time"
)

type HTTPObserver struct {
	registry *Registry
}

func NewHTTPObserver(registry *Registry) HTTPObserver {
	return HTTPObserver{registry: registry}
}

func (o HTTPObserver) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if o.registry == nil {
		return
	}
	statusLabel := strconv.Itoa(status)
	if o.registry.HTTPRequestTotal != nil {
		o.registry.HTTPRequestTotal.WithLabelValues(method, route, statusLabel).Inc()
	}
	if o.registry.HTTPRequestDuration != nil {
		o.registry.HTTPRequestDuration.WithLabelValues(method, route, statusLabel).Observe(duration.Seconds())
	}
}

func (o HTTPObserver) ObserveHTTPPanic(method, route string) {
	if o.registry == nil || o.registry.HTTPPanicTotal == nil {
		return
	}
	o.registry.HTTPPanicTotal.WithLabelValues(method, route).Inc()
}
