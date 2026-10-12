package events

import (
	"testing"
	"time"
)

func TestBotAccountNotificationsAreCoalesced(t *testing.T) {
	s := NewBotAccountsService()
	frames, unsubscribe := s.Subscribe(10)
	defer unsubscribe()
	for range 100 {
		s.PublishChanged()
	}
	select {
	case frame := <-frames:
		payload, ok := frame.Data.(GenericPayload)
		if !ok || payload.EventType != "bot_accounts.changed" || frame.Type != "events.received" {
			t.Fatal(frame)
		}
	case <-time.After(time.Second):
		t.Fatal("missing directory notification")
	}
	select {
	case frame := <-frames:
		t.Fatalf("duplicate notification: %+v", frame)
	default:
	}
}
