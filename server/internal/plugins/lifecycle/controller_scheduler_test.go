package lifecycle

import (
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func TestSchedulerPayloadFieldsPreservesJobPayloadForProtocolProjection(t *testing.T) {
	t.Parallel()

	fields := schedulerPayloadFields(scheduler.Job{
		JobID:   "raylea.subscription-hub/subscription-hub-check",
		Payload: json.RawMessage(`{"action":"check_subscriptions","limit":10}`),
	})

	if got := fields["action"]; got != "check_subscriptions" {
		t.Fatalf("action = %#v, want check_subscriptions", got)
	}
	payload, ok := fields["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload = %#v, want map", fields["payload"])
	}
	if got := payload["limit"]; got != float64(10) {
		t.Fatalf("payload limit = %#v, want 10", got)
	}
	if _, exists := fields["job_id"]; exists {
		t.Fatalf("scheduler payload leaked non-contract job_id: %#v", fields)
	}
}
