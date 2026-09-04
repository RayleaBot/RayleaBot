package rayleabot

import (
	"context"
	"errors"
	"time"
)

func (event *EventContext) finishAction(id string) {
	event.actionMu.Lock()
	defer event.actionMu.Unlock()
	if done := event.actions[id]; done != nil {
		delete(event.actions, id)
		close(done)
	}
}

func (event *EventContext) writeTerminal(frame protocolFrame) error {
	event.actionMu.Lock()
	if !event.terminal.CompareAndSwap(false, true) {
		event.actionMu.Unlock()
		return errors.New("rayleabot: terminal response already sent or event is closing")
	}
	pending := make([]<-chan struct{}, 0, len(event.actions))
	for _, done := range event.actions {
		pending = append(pending, done)
	}
	event.actionMu.Unlock()

	timer := time.NewTimer(event.client.actionTimeout)
	defer timer.Stop()
	for _, done := range pending {
		select {
		case <-done:
		case <-event.client.done:
			return context.Canceled
		case <-timer.C:
			event.client.retireEvent(event)
			return errors.New("rayleabot: local actions did not settle; terminal response withheld")
		}
	}
	select {
	case <-event.client.done:
		return context.Canceled
	default:
		return event.client.writer.write(frame)
	}
}
