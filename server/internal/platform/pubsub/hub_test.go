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

func TestHubCloseEndsCurrentAndFutureSubscriptions(t *testing.T) {
	var hub Hub[int]
	current, unsubscribe := hub.Subscribe(1)
	hub.Close()
	hub.Close()
	unsubscribe()
	hub.Publish(1)
	hub.PublishReplace(2)
	next, unsubscribeNext := hub.Subscribe(1)
	defer unsubscribeNext()
	for _, channel := range []<-chan int{current, next} {
		select {
		case _, open := <-channel:
			if open {
				t.Fatal("closed hub delivered a value")
			}
		default:
			t.Fatal("subscription survived hub shutdown")
		}
	}
	if hub.SubscriberCount() != 0 {
		t.Fatal("closed hub retained subscribers")
	}
}

func TestPublishReplaceEachIsolatesLatestValues(t *testing.T) {
	var hub Hub[[]int]
	first, unsubscribeFirst := hub.Subscribe(1)
	defer unsubscribeFirst()
	second, unsubscribeSecond := hub.Subscribe(1)
	defer unsubscribeSecond()
	hub.PublishReplaceEach(func() []int { return []int{1} })
	hub.PublishReplaceEach(func() []int { return []int{2} })
	a, b := <-first, <-second
	if a[0] != 2 || b[0] != 2 {
		t.Fatal("full subscriptions did not retain latest values")
	}
	a[0] = 3
	if b[0] != 2 {
		t.Fatal("mutable payload was shared between subscribers")
	}
}
