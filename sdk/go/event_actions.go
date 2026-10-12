package rayleabot

import (
	"context"
	"errors"
	"time"
)

// Address names a conversation by chat protocol, bot account and target.
// It stays valid when the adapter instance serving the account is replaced
// or renamed, so plugins store it for later delivery.
type Address struct {
	SourceProtocol string `json:"source_protocol"`
	BotID          string `json:"bot_id"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
}

// Address returns the current chat event's persistent delivery address.
func (event *EventContext) Address() (Address, bool) {
	current := event.Event
	if (current.SourceProtocol != "onebot11" && current.SourceProtocol != "qqofficial") || current.BotID == "" || current.Target.Type == "" || current.Target.ID == "" {
		return Address{}, false
	}
	return Address{SourceProtocol: current.SourceProtocol, BotID: current.BotID, TargetType: current.Target.Type, TargetID: current.Target.ID}, true
}

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
