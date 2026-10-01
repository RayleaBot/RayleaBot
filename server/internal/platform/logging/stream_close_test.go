package logging

import "testing"

func TestCloseEndsSubscriptionsAndRejectsNewSubscribers(t *testing.T) {
	s := NewStream(4)
	updates, unsubscribe := s.Subscribe(1)
	defer unsubscribe()
	s.Close()
	s.Close()
	if _, open := <-updates; open {
		t.Fatal("subscription remained open")
	}
	if s.SubscriberCount() != 0 {
		t.Fatal("subscriber retained")
	}
	late, cancel := s.Subscribe(1)
	defer cancel()
	if _, open := <-late; open {
		t.Fatal("closed stream accepted a new subscriber")
	}
}
