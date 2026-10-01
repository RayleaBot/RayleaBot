package chatpolicy

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"testing"
)

type statsOrderEnricher struct {
	counted *int
	t       *testing.T
}

func (e statsOrderEnricher) EnrichEventMetadata(_ context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	if *e.counted != 1 {
		e.t.Fatal("metadata/routing ran before ingress counting")
	}
	return event
}

func TestIngressCountsBeforeRoutingEvenWhenProcessingIsCancelled(t *testing.T) {
	count := 0
	ingress := NewIngress(IngressDeps{MessageReceived: func(chatevent.NormalizedEvent) { count++ }, MetadataEnricher: statsOrderEnricher{&count, t}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ingress.HandleAdapterEvent(ctx, chatevent.NormalizedEvent{EventType: "message.group"})
	if count != 1 {
		t.Fatal("cancelled processing suppressed received count")
	}
	ingress.StopAdmission()
	ingress.HandleAdapterEvent(t.Context(), chatevent.NormalizedEvent{EventType: "message.private"})
	if count != 2 {
		t.Fatal("host ingress during drain was not observed")
	}
}
