package dispatch

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestMessageSubscriptionsFollowSwapAndDeregister(t *testing.T) {
	d := New(nil, nil, nil, 8)
	t.Cleanup(d.Close)
	d.Register("wildcard", &fakeDeliverer{}, []string{"*", "message.group"}, nil, 2, MessagePolicy{Priority: 2})
	d.Register("specific", &fakeDeliverer{}, []string{"message.group"}, nil, 2)
	event := testEvent()
	check := func(want ...string) {
		t.Helper()
		results := d.Dispatch(t.Context(), event, "")
		if len(results) != len(want) {
			t.Fatalf("routes=%+v want=%v", results, want)
		}
		for i, result := range results {
			if result.PluginID != want[i] || !waitCompletion(t, result).Success {
				t.Fatalf("route %d=%+v", i, result)
			}
		}
	}
	check("wildcard", "specific")
	drain, err := d.SwapPlugin("specific", &fakeDeliverer{}, []string{"message.private"}, nil, 2, MessagePolicy{Priority: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := drain.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	check("wildcard")
	event.EventType, event.Target = "message.private", &chatevent.Target{Type: "private", ID: "2"}
	check("specific", "wildcard")
	d.Deregister("specific")
	check("wildcard")
	d.Deregister("wildcard")
	check()
}
