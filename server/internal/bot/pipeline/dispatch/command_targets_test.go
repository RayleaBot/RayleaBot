package dispatch

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestResolvedCommandTargetsDecideDelivery(t *testing.T) {
	note := []plugins.Command{{Name: "体力"}}
	setup := func(t *testing.T) (*Dispatcher, *fakeDeliverer, *fakeDeliverer) {
		d := New(slog.Default(), &fakeSender{}, nil, 16)
		t.Cleanup(d.Close)
		genshin := &fakeDeliverer{delivery: plugins.Delivery{Result: map[string]any{}}, started: make(chan chatevent.Event, 1)}
		starrail := &fakeDeliverer{delivery: plugins.Delivery{Result: map[string]any{}}, started: make(chan chatevent.Event, 1)}
		d.Register("genshin", genshin, []string{"message.group"}, note, 1)
		d.Register("starrail", starrail, []string{"message.group"}, note, 1)
		return d, genshin, starrail
	}
	command := func(targets ...chatevent.CommandTarget) chatevent.Event {
		event := testEvent()
		event.PayloadFields = map[string]any{"command": "体力", "args": []string{}, "kept": true}
		event.CommandResolved, event.CommandTargets = true, targets
		return event
	}

	t.Run("only the resolved target receives the command, with its own parse", func(t *testing.T) {
		d, genshin, starrail := setup(t)
		results := d.Dispatch(context.Background(), command(chatevent.CommandTarget{PluginID: "starrail", Command: "体力", Args: []string{"100000001"}}), "体力")
		if len(results) != 1 || results[0].PluginID != "starrail" {
			t.Fatalf("results = %+v", results)
		}
		delivered := waitForStartedEvent(t, starrail.started)
		if delivered.PayloadFields["command"] != "体力" || !reflect.DeepEqual(delivered.PayloadFields["args"], []string{"100000001"}) || delivered.PayloadFields["kept"] != true {
			t.Fatalf("payload = %#v", delivered.PayloadFields)
		}
		if genshin.eventCount() != 0 {
			t.Fatal("a plugin declaring the same command name received a command addressed elsewhere")
		}
	})
	t.Run("an addressed command is not broadcast when its target cannot take it", func(t *testing.T) {
		d, genshin, starrail := setup(t)
		genshin.setUnavailable()
		if results := d.Dispatch(context.Background(), command(chatevent.CommandTarget{PluginID: "genshin", Command: "体力"}), "体力"); len(results) != 0 {
			t.Fatalf("results = %+v", results)
		}
		if starrail.eventCount() != 0 {
			t.Fatal("the command reached a plugin it was not addressed to")
		}
	})
	t.Run("a command nobody declares still reaches subscribers", func(t *testing.T) {
		d, genshin, starrail := setup(t)
		if results := d.Dispatch(context.Background(), command(), "unknown"); len(results) != 2 {
			t.Fatalf("results = %+v", results)
		}
		waitForStartedEvent(t, genshin.started)
		waitForStartedEvent(t, starrail.started)
	})
}
