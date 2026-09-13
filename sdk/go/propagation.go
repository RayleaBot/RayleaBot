package rayleabot

import "errors"

type Propagation string

const (
	PropagationContinue Propagation = "continue"
	PropagationStop     Propagation = "stop"
)

// ResultWithPropagation completes this message and controls later priorities.
// Peers in the current priority layer have already been admitted.
func (event *EventContext) ResultWithPropagation(data any, propagation Propagation) error {
	if propagation != PropagationContinue && propagation != PropagationStop {
		return errors.New("rayleabot: invalid propagation result")
	}
	if event.Event.EventType != "message.private" && event.Event.EventType != "message.group" {
		return errors.New("rayleabot: propagation requires a message event")
	}
	return event.result(data, string(propagation))
}
