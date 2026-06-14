package metrics

type WebhookReplayObserver struct {
	registry *Registry
}

func NewWebhookReplayObserver(registry *Registry) WebhookReplayObserver {
	return WebhookReplayObserver{registry: registry}
}

func (a WebhookReplayObserver) IncReplayObserved(outcome string) {
	if a.registry == nil || a.registry.WebhookReplayObserved == nil {
		return
	}
	a.registry.WebhookReplayObserved.WithLabelValues(outcome).Inc()
}
