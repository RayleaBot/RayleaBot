// Package pubsub provides the shared in-process fan-out hub used by every
// subscription surface (logs, tasks, console, plugin catalog, WS event
// services). Publishing never blocks; unsubscribing closes the channel.
package pubsub

import "sync"

// Hub broadcasts values to all current subscribers. The zero value is ready
// to use.
type Hub[T any] struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]chan T
}

func NewHub[T any]() *Hub[T] {
	return &Hub[T]{}
}

// Subscribe registers a subscriber with the given buffer size (minimum 1)
// and returns the receive channel plus an idempotent unsubscribe function
// that closes the channel.
func (h *Hub[T]) Subscribe(buffer int) (<-chan T, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan T, buffer)
	h.mu.Lock()
	if h.subscribers == nil {
		h.subscribers = make(map[uint64]chan T)
	}
	id := h.nextID
	h.nextID++
	h.subscribers[id] = ch
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		subscriber, ok := h.subscribers[id]
		if !ok {
			return
		}
		delete(h.subscribers, id)
		close(subscriber)
	}
}

func (h *Hub[T]) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}

// Publish sends value to every subscriber; a subscriber with a full buffer
// misses the value.
func (h *Hub[T]) Publish(value T) {
	h.PublishEach(func() T { return value })
}

// PublishEach behaves like Publish but computes a fresh value per
// subscriber, for payloads that must not be shared across receivers.
func (h *Hub[T]) PublishEach(next func() T) {
	for _, subscriber := range h.snapshotSubscribers() {
		select {
		case subscriber <- next():
		default:
		}
	}
}

// PublishReplace sends value to every subscriber; when a subscriber's
// buffer is full the oldest buffered value is evicted and the send is
// retried once, so laggards keep the newest data.
func (h *Hub[T]) PublishReplace(value T) {
	for _, subscriber := range h.snapshotSubscribers() {
		select {
		case subscriber <- value:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- value:
			default:
			}
		}
	}
}

func (h *Hub[T]) snapshotSubscribers() []chan T {
	h.mu.RLock()
	defer h.mu.RUnlock()
	subscribers := make([]chan T, 0, len(h.subscribers))
	for _, subscriber := range h.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	return subscribers
}
