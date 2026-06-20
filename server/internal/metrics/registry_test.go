package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRegistersAllFormalMetrics(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New returned nil registry")
	}
	r.EventPipelineStage.WithLabelValues("adapter", "accepted").Inc()
	r.PluginState.WithLabelValues("running").Set(2)
	r.DispatcherDropTotal.WithLabelValues("builtin.echo", "queue_full").Inc()
	r.RenderQueueDepth.Set(3)
	r.WebhookReplayObserved.WithLabelValues("rejected").Inc()

	server := httptest.NewServer(r.HTTPHandler())
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("metrics request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	text := string(body)

	expected := []string{
		`raylea_event_pipeline_stage_total{outcome="accepted",stage="adapter"} 1`,
		`raylea_plugin_state{state="running"} 2`,
		`raylea_dispatcher_drop_total{plugin_id="builtin.echo",reason="queue_full"} 1`,
		`raylea_render_queue_depth 3`,
		`raylea_plugin_webhook_replay_observed_total{outcome="rejected"} 1`,
	}
	for _, line := range expected {
		if !strings.Contains(text, line) {
			t.Errorf("expected exposition to contain %q\nactual:\n%s", line, text)
		}
	}
}

func TestRegistryNilHTTPHandler(t *testing.T) {
	var r *Registry
	w := httptest.NewRecorder()
	r.HTTPHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from nil registry, got %d", w.Code)
	}
}
