package render

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// recordingRenderMetrics captures every render outcome and queue depth
// signal so TestServiceRenderRecordsMetrics can assert the observer hooks
// used by /api/system/metrics actually fire.
type recordingRenderMetrics struct {
	mu              sync.Mutex
	durations       []renderMetricSample
	maxQueueDepth   int
	queueDepthCalls int
}

type renderMetricSample struct {
	outcome  string
	duration time.Duration
}

func (m *recordingRenderMetrics) SetRenderQueueDepth(depth int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueDepthCalls++
	if depth > m.maxQueueDepth {
		m.maxQueueDepth = depth
	}
}

func (m *recordingRenderMetrics) ObserveRenderDuration(outcome string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations = append(m.durations, renderMetricSample{outcome: outcome, duration: duration})
}

// TestServiceRenderRecordsMetrics verifies the render service drives the
// configured MetricsObserver for both successful renders and cache hits.
// The /api/system/metrics contract advertises render_queue_depth and
// render_duration_seconds; this test guards the actual write paths.
func TestServiceRenderRecordsMetrics(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-metrics")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	metrics := &recordingRenderMetrics{}
	service.SetMetricsObserver(metrics)

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气", "usage": "/weather <城市>"},
			},
		},
	}

	if _, err := service.Render(context.Background(), request); err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if _, err := service.Render(context.Background(), request); err != nil {
		t.Fatalf("second Render: %v", err)
	}

	// SetRenderQueueDepth runs in a goroutine; give it a moment.
	time.Sleep(50 * time.Millisecond)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	if len(metrics.durations) != 2 {
		t.Fatalf("durations = %d, want 2", len(metrics.durations))
	}
	outcomes := map[string]int{}
	for _, sample := range metrics.durations {
		outcomes[sample.outcome]++
	}
	if outcomes["succeeded"] != 1 {
		t.Fatalf("succeeded count = %d, want 1", outcomes["succeeded"])
	}
	if outcomes["cache_hit"] != 1 {
		t.Fatalf("cache_hit count = %d, want 1", outcomes["cache_hit"])
	}
	if metrics.queueDepthCalls == 0 {
		t.Fatal("expected at least one queue-depth update")
	}
	if metrics.maxQueueDepth < 1 {
		t.Fatalf("maxQueueDepth = %d, want >= 1", metrics.maxQueueDepth)
	}
}
