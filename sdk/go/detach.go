package rayleabot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Detach moves this event to the background with event.detach and returns the
// background deadline. The host completes the delivery with result as a
// successful terminal would: a management.action caller receives it, message
// layering continues and a scheduler trigger counts as delivered. result must
// encode to a JSON object; nil sends an empty result.
//
// The event keeps its request ID and origin, so Actions keeps working until
// the handler returns or the background deadline passes. The handler context
// then ends at that deadline instead of the event deadline, and the event no
// longer holds a concurrency permit. SendText, Send and Reply send an ordinary
// message action followed by a terminal Result; Result and Fail end the event.
// Conversation waits (Ask, SessionWait) are unavailable after Detach. The host
// allows one Detach per event for message.private, message.group,
// scheduler.trigger and management.action, and rejects it with a retryable
// platform.rate_limited ActionError while the plugin already holds its limit
// of background events; a rejected event stays in the foreground.
func (event *EventContext) Detach(ctx context.Context, result any) (time.Time, error) {
	return event.detach(ctx, result, "")
}

// DetachWithPropagation is Detach for a message event that also decides later
// priority layers, like ResultWithPropagation.
func (event *EventContext) DetachWithPropagation(ctx context.Context, result any, propagation Propagation) (time.Time, error) {
	if propagation != PropagationContinue && propagation != PropagationStop {
		return time.Time{}, errors.New("rayleabot: invalid propagation result")
	}
	if event.Event.EventType != "message.private" && event.Event.EventType != "message.group" {
		return time.Time{}, errors.New("rayleabot: propagation requires a message event")
	}
	return event.detach(ctx, result, string(propagation))
}

func (event *EventContext) detach(ctx context.Context, result any, propagation string) (time.Time, error) {
	if event.detached.Load() {
		return time.Time{}, errors.New("rayleabot: event is already in the background")
	}
	request := map[string]json.RawMessage{}
	if result != nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return time.Time{}, fmt.Errorf("rayleabot: marshal detach result: %w", err)
		}
		switch trimmed := bytes.TrimSpace(encoded); {
		case string(trimmed) == "null":
		case len(trimmed) > 0 && trimmed[0] == '{':
			request["result"] = encoded
		default:
			return time.Time{}, errors.New("rayleabot: detach result must be a JSON object")
		}
	}
	if propagation != "" {
		request["propagation"] = json.RawMessage(`"` + propagation + `"`)
	}
	var response struct {
		DeadlineAtMs int64 `json:"deadline_at_ms"`
	}
	if err := event.Actions().Call(ctx, "event.detach", request, &response); err != nil {
		return time.Time{}, err
	}
	if response.DeadlineAtMs <= 0 {
		return time.Time{}, errors.New("rayleabot: event.detach response has no deadline")
	}
	deadline := time.UnixMilli(response.DeadlineAtMs)
	event.detached.Store(true)
	if event.lifetime != nil {
		event.lifetime.extend(deadline)
	}
	if event.release != nil {
		event.release()
	}
	return deadline, nil
}

// Deadline is the current host deadline of this event: the event frame
// deadline, or the background deadline after Detach. The handler context ends
// at the same time.
func (event *EventContext) Deadline() time.Time {
	if event.lifetime == nil {
		return time.Time{}
	}
	deadline, _ := event.lifetime.Deadline()
	return deadline
}

// Detached reports whether Detach moved this event to the background.
func (event *EventContext) Detached() bool { return event.detached.Load() }

// sendDetached replaces a terminal message action after Detach: the message is
// an ordinary action and a terminal result ends the background event.
func (event *EventContext) sendDetached(request MessageSendRequest) error {
	ctx := context.Context(context.Background())
	if event.lifetime != nil {
		ctx = event.lifetime
	}
	if _, err := event.Actions().MessageSend(ctx, request); err != nil {
		return err
	}
	return event.Result(nil)
}

// eventLifetime is the handler context. It ends at the host deadline of the
// event or when the runtime shuts down; Detach moves the deadline once to the
// background deadline. Deadline reports the current value, so a child context
// derived before Detach with a longer timeout follows the extended deadline.
type eventLifetime struct {
	parent context.Context
	done   chan struct{}
	stop   func() bool

	mu       sync.Mutex
	deadline time.Time
	err      error
	timer    *time.Timer
}

func newEventLifetime(parent context.Context, deadline time.Time) *eventLifetime {
	lifetime := &eventLifetime{parent: parent, done: make(chan struct{}), deadline: deadline}
	lifetime.mu.Lock()
	defer lifetime.mu.Unlock()
	lifetime.timer = time.AfterFunc(time.Until(deadline), lifetime.expire)
	lifetime.stop = context.AfterFunc(parent, func() { lifetime.finish(parent.Err()) })
	return lifetime
}

func (lifetime *eventLifetime) Deadline() (time.Time, bool) {
	lifetime.mu.Lock()
	defer lifetime.mu.Unlock()
	return lifetime.deadline, true
}

func (lifetime *eventLifetime) Done() <-chan struct{} { return lifetime.done }

func (lifetime *eventLifetime) Err() error {
	lifetime.mu.Lock()
	defer lifetime.mu.Unlock()
	return lifetime.err
}

func (lifetime *eventLifetime) Value(key any) any { return lifetime.parent.Value(key) }

func (lifetime *eventLifetime) expire() {
	lifetime.mu.Lock()
	if lifetime.err == nil {
		if remaining := time.Until(lifetime.deadline); remaining > 0 {
			// Detach moved the deadline, or the wall clock stepped back.
			lifetime.timer.Reset(remaining)
			lifetime.mu.Unlock()
			return
		}
	}
	lifetime.mu.Unlock()
	lifetime.finish(context.DeadlineExceeded)
}

func (lifetime *eventLifetime) finish(err error) {
	lifetime.mu.Lock()
	defer lifetime.mu.Unlock()
	if lifetime.err != nil {
		return
	}
	lifetime.err = err
	lifetime.timer.Stop()
	if lifetime.stop != nil {
		lifetime.stop()
	}
	close(lifetime.done)
}

func (lifetime *eventLifetime) extend(deadline time.Time) {
	lifetime.mu.Lock()
	defer lifetime.mu.Unlock()
	if lifetime.err != nil {
		return
	}
	lifetime.deadline = deadline
	lifetime.timer.Reset(time.Until(deadline))
}
