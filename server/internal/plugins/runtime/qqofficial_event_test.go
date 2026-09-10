package runtime

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestQQNativePayloadReachesPluginFrame(t *testing.T) {
	event, ok := qqofficial.NormalizeDispatch("event-fixture", "GROUP_AT_MESSAGE_CREATE", []byte(`{"id":"message-fixture","content":"fixture","group_openid":"group-fixture","author":{"member_openid":"member-fixture"},"timestamp":"2026-09-07T12:00:00Z"}`))
	if !ok {
		t.Fatal("invalid fixture")
	}
	frame := BuildEventFrame(chatevent.FromAdapter(event), "request-fixture")
	encoded, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	payload := wire["event"].(map[string]any)["payload"].(map[string]any)
	if !reflect.DeepEqual(payload["qq_official"], event.PayloadFields["qq_official"]) {
		t.Fatalf("native payload = %#v, want %#v", payload["qq_official"], event.PayloadFields["qq_official"])
	}
}
