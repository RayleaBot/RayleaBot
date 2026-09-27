package plugins

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

type Error struct {
	failureReported atomic.Bool
	Code            string
	Message         string
	Details         map[string]any
	Err             error
}

// FailureReported reports whether the runtime emitted the owning failure record.
func (e *Error) FailureReported() bool { return e.failureReported.Load() }

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// MarkFailureReported is called by the owning runtime before publishing the failure.
func (e *Error) MarkFailureReported() { e.failureReported.Store(true) }

type Delivery struct {
	RequestID    string
	Propagation  string
	Action       *chatevent.MessageCommand
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
	ErrorDetails map[string]any
	// Detached is set when the plugin moved the event to the background with
	// event.detach. Result and Propagation then hold the detach result, and the
	// event itself ends later.
	Detached *DetachedEvent
}

// DetachedEvent reports how a background event ends after its delivery has
// already completed with the detach result.
type DetachedEvent struct {
	once sync.Once
	done chan struct{}
	err  error
}

func NewDetachedEvent() *DetachedEvent { return &DetachedEvent{done: make(chan struct{})} }

// Done is closed when the event ends by its terminal, deadline or runtime stop.
func (e *DetachedEvent) Done() <-chan struct{} { return e.done }

// Err waits for the end and returns nil for a successful terminal.
func (e *DetachedEvent) Err() error {
	<-e.done
	return e.err
}

// Finish records the end; the owning runtime calls it once per event.
func (e *DetachedEvent) Finish(err error) {
	e.once.Do(func() {
		e.err = err
		close(e.done)
	})
}
