package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/runtime"
)

type dispatcherRuntimeClient struct {
	dispatcher *dispatch.Dispatcher
}

func newDispatcherRuntimeClient(dispatcher *dispatch.Dispatcher) *dispatcherRuntimeClient {
	return &dispatcherRuntimeClient{dispatcher: dispatcher}
}

func (c *dispatcherRuntimeClient) Snapshot() runtime.Snapshot {
	if c == nil || c.dispatcher == nil {
		return runtime.Snapshot{State: runtime.StateStopped}
	}
	if len(c.dispatcher.PluginIDs()) == 0 {
		return runtime.Snapshot{State: runtime.StateStopped}
	}
	return runtime.Snapshot{State: runtime.StateRunning}
}

func (c *dispatcherRuntimeClient) DeliverEvent(ctx context.Context, event runtime.Event) (runtime.Delivery, error) {
	if c == nil || c.dispatcher == nil {
		return runtime.Delivery{}, &runtime.Error{
			Code:    "platform.invalid_request",
			Message: "dispatcher runtime is not available",
		}
	}

	commandName := ""
	if event.PayloadFields != nil {
		if value, ok := event.PayloadFields["command"].(string); ok {
			commandName = strings.TrimSpace(value)
		}
	}

	results := c.dispatcher.Dispatch(ctx, event, commandName)
	if len(results) == 0 {
		return runtime.Delivery{}, &runtime.Error{
			Code:    "platform.invalid_request",
			Message: "no eligible plugin runtime accepted the event",
		}
	}

	delivered := 0
	for _, result := range results {
		if result.Outcome == dispatch.OutcomeDelivered {
			delivered++
		}
	}
	if delivered == 0 {
		return runtime.Delivery{}, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "all plugin deliveries were dropped before runtime handling",
		}
	}

	return runtime.Delivery{
		RequestID: fmt.Sprintf("dispatch_%d", time.Now().UnixNano()),
		Result: map[string]any{
			"delivered_plugins": delivered,
		},
	}, nil
}
