package rayleabot

import (
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

func TestManagementAccountContext(t *testing.T) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(`{"page_id":"records","bot":{"source_protocol":"qqofficial","bot_id":"opaque:id"},"payload":{"bot":{"source_protocol":"onebot11","bot_id":"spoof"}}}`), &payload); err != nil {
		t.Fatal(err)
	}
	event := Event{EventType: "management.action", SourceProtocol: "management", Payload: payload}
	bot, ok := event.ManagementBot()
	if !ok || bot.SourceProtocol != "qqofficial" || bot.BotID != "opaque:id" {
		t.Fatalf("context=%+v %v", bot, ok)
	}
	encoded, err := json.Marshal(bot)
	if err != nil || string(encoded) != `{"source_protocol":"qqofficial","bot_id":"opaque:id"}` {
		t.Fatalf("bot JSON=%s %v", encoded, err)
	}
	for _, test := range []Event{
		{EventType: "message.group", SourceProtocol: "onebot11", Payload: payload},
		{EventType: "management.action", SourceProtocol: "platform", Payload: payload},
		{EventType: "management.action", SourceProtocol: "management", Payload: map[string]any{"payload": payload}},
		{EventType: "management.action", SourceProtocol: "management", Payload: map[string]any{"page_id": "records", "bot": map[string]any{"source_protocol": "unknown", "bot_id": "account"}}},
		{EventType: "management.action", SourceProtocol: "management", Payload: map[string]any{"page_id": "records", "bot": map[string]any{"source_protocol": "onebot11", "bot_id": 1}}},
	} {
		if _, ok := test.ManagementBot(); ok {
			t.Fatalf("accepted context=%+v", test)
		}
	}
	var request ServiceRequest
	if err := json.Unmarshal([]byte(`{"origin":{"event_type":"management.action","source_protocol":"management","source_adapter":"management.ui","bot":{"source_protocol":"qqofficial","bot_id":"opaque:id"}}}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.Origin.Bot == nil || *request.Origin.Bot != (pluginwire.BotAccountRef(bot)) {
		t.Fatal(request.Origin)
	}
}
