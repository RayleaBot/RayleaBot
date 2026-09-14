package runtime

import (
	"encoding/json"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

func TestTerminalResultCarriesPropagation(t *testing.T) {
	delivery, done, err := decodeTerminalDelivery("event", pluginwire.Frame{Type: "result", RequestID: "event", Status: "success", Data: json.RawMessage(`{}`), Propagation: "stop"})
	if err != nil || !done || delivery.Propagation != "stop" {
		t.Fatalf("delivery=%#v done=%v err=%v", delivery, done, err)
	}
}

func TestPropagationRejectsNonterminalAndNonmessageContext(t *testing.T) {
	m := &Manager{}
	if _, err := m.routeLocalActionFrameLocked(nil, pluginwire.Frame{Type: "action", Propagation: "stop"}); err == nil {
		t.Fatal("nonterminal propagation accepted")
	}
	if err := m.routeTerminalFrameLocked(&eventSession{event: chatevent.Event{EventType: "scheduler.trigger"}}, pluginwire.Frame{Type: "result", Propagation: "stop"}); err == nil {
		t.Fatal("scheduler propagation accepted")
	}
}
