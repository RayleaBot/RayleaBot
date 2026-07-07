package pubsub

import (
	"testing"
	"time"
)

func TestPublishEachSerializesWithUnsubscribe(t *testing.T) {
	var hub Hub[int]
	_, unsubscribe := hub.Subscribe(1)

	entered := make(chan struct{})
	release := make(chan struct{})
	published := make(chan any, 1)

	go func() {
		defer func() {
			published <- recover()
		}()
		hub.PublishEach(func() int {
			close(entered)
			<-release
			return 1
		})
	}()

	<-entered
	if hub.mu.TryLock() {
		hub.mu.Unlock()
		t.Fatal("PublishEach released the hub lock before sending to subscribers")
	}

	unsubscribed := make(chan struct{})
	go func() {
		unsubscribe()
		close(unsubscribed)
	}()

	close(release)
	if recovered := <-published; recovered != nil {
		t.Fatalf("PublishEach panicked: %v", recovered)
	}

	select {
	case <-unsubscribed:
	case <-time.After(time.Second):
		t.Fatal("unsubscribe did not finish after publish completed")
	}
}
