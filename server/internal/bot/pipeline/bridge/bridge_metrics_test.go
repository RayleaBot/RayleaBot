package bridge

import (
	"context"
	"sync"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
)

type recordingMetrics struct {
	mu       sync.Mutex
	pipeline map[string]int
	ignored  int
}

func (m *recordingMetrics) IncEventPipelineStage(stage, outcome string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pipeline == nil {
		m.pipeline = make(map[string]int)
	}
	m.pipeline[stage+"\x00"+outcome]++
}

func (m *recordingMetrics) IncBridgeIgnored() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ignored++
}

func (m *recordingMetrics) pipelineCount(stage, outcome string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pipeline[stage+"\x00"+outcome]
}

func (m *recordingMetrics) ignoredCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ignored
}

func TestBridgeMetricsObserverIncrementsOnEachOutcome(t *testing.T) {
	t.Parallel()

	deliveredDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "weather",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	}
	deliveredBridge := testBridge(deliveredDispatcher)
	deliveredMetrics := &recordingMetrics{}
	deliveredBridge.SetMetricsObserver(deliveredMetrics)
	if outcome := deliveredBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent()); outcome != chatevent.DeliveryOutcomeDelivered {
		t.Fatalf("expected delivered outcome, got %q", outcome)
	}
	if got := deliveredMetrics.pipelineCount("bridge", string(chatevent.DeliveryOutcomeDelivered)); got != 1 {
		t.Fatalf("expected bridge:delivered=1, got %d", got)
	}

	ignoredBridge := testBridge(&recordingDispatcher{deliverable: false})
	ignoredMetrics := &recordingMetrics{}
	ignoredBridge.SetMetricsObserver(ignoredMetrics)
	if outcome := ignoredBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent()); outcome != chatevent.DeliveryOutcomeIgnored {
		t.Fatalf("expected ignored outcome, got %q", outcome)
	}
	if got := ignoredMetrics.pipelineCount("bridge", string(chatevent.DeliveryOutcomeIgnored)); got != 1 {
		t.Fatalf("expected bridge:ignored=1, got %d", got)
	}
	if got := ignoredMetrics.ignoredCount(); got != 1 {
		t.Fatalf("expected ignored counter to fire once, got %d", got)
	}

	errorDispatcher := &recordingDispatcher{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID:  "weather",
			Outcome:   dispatch.OutcomeError,
			ErrorCode: "plugin.internal_error",
		}},
	}
	errorBridge := testBridge(errorDispatcher)
	errorMetrics := &recordingMetrics{}
	errorBridge.SetMetricsObserver(errorMetrics)
	if outcome := errorBridge.HandleAdapterEvent(context.Background(), supportedAdapterEvent()); outcome != chatevent.DeliveryOutcomeError {
		t.Fatalf("expected error outcome, got %q", outcome)
	}
	if got := errorMetrics.pipelineCount("bridge", string(chatevent.DeliveryOutcomeError)); got != 1 {
		t.Fatalf("expected bridge:error=1, got %d", got)
	}
}
