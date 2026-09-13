package rayleabot

import (
	"testing"
	"time"
)

func TestMessageResultPropagationAndNilData(t *testing.T) {
	sink := make(actionFrameSink, 1)
	event := &EventContext{RequestID: "fixture", Event: Event{EventType: "message.group"}, client: newRuntimeClient(sink, time.Second)}
	if err := event.ResultWithPropagation(nil, PropagationStop); err != nil {
		t.Fatal(err)
	}
	frame := <-sink
	if frame.Propagation != "stop" || string(frame.Data) != "{}" {
		t.Fatalf("terminal=%#v", frame)
	}
	if err := event.Result(nil); err == nil {
		t.Fatal("event completed twice")
	}
}

func TestInvalidPropagationDoesNotCloseEvent(t *testing.T) {
	for _, eventType := range []string{"scheduler.trigger", "message.group"} {
		sink := make(actionFrameSink, 1)
		event := &EventContext{RequestID: "fixture", Event: Event{EventType: eventType}, client: newRuntimeClient(sink, time.Second)}
		propagation := PropagationStop
		if eventType == "message.group" {
			propagation = "unknown"
		}
		if err := event.ResultWithPropagation(nil, propagation); err == nil {
			t.Fatal("invalid propagation accepted")
		}
		if err := event.Result(nil); err != nil {
			t.Fatal("invalid argument closed the event")
		}
	}
}
