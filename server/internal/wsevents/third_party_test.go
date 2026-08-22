package wsevents

import "testing"

func TestThirdPartyAccountChangedEventFrame(t *testing.T) {
	frame := thirdPartyAccountChangedEventFrame()
	if frame.Channel != "events" || frame.Type != "events.received" {
		t.Fatalf("unexpected envelope: %#v", frame)
	}
	payload, ok := frame.Data.(GenericPayload)
	if !ok {
		t.Fatalf("payload type = %T", frame.Data)
	}
	if payload.EventType != "third_party.account.changed" || payload.Summary == "" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
