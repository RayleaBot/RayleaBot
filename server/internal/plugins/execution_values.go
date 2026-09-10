package plugins

import (
	"fmt"
	"sync/atomic"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
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
	Action       *chatevent.MessageCommand
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
	ErrorDetails map[string]any
}
