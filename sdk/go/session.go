package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

type SessionRef struct {
	SessionID   string `json:"session_id"`
	Scope       string `json:"scope"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}
type SessionWaitOptions struct {
	SessionID      string
	Scope          string
	Timeout        time.Duration
	NotifyOnExpire *bool
}

func (actions *Actions) SessionWait(ctx context.Context, options SessionWaitOptions) (SessionRef, error) {
	if options.Timeout < 0 || options.Timeout%time.Second != 0 || options.Timeout > 600*time.Second {
		return SessionRef{}, errors.New("rayleabot: session timeout must be zero or whole seconds up to 600")
	}
	request := struct {
		SessionID      string `json:"session_id,omitempty"`
		Scope          string `json:"scope,omitempty"`
		TimeoutSeconds int64  `json:"timeout_seconds,omitempty"`
		NotifyOnExpire *bool  `json:"notify_on_expire,omitempty"`
	}{SessionID: options.SessionID, Scope: options.Scope, TimeoutSeconds: int64(options.Timeout / time.Second), NotifyOnExpire: options.NotifyOnExpire}
	var data json.RawMessage
	if err := actions.Call(ctx, "session.wait", request, &data); err != nil {
		return SessionRef{}, err
	}
	if err := pluginwire.ValidateActionResult("session.wait", data); err != nil {
		return SessionRef{}, err
	}
	var ref SessionRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return SessionRef{}, err
	}
	actions.event.client.callbacks.registered(ref, sessionRoute(actions.event, ref.Scope))
	return ref, nil
}

func (actions *Actions) SessionFinish(ctx context.Context, id string) (bool, error) {
	if actions != nil && actions.event != nil && actions.event.client != nil {
		actions.event.client.callbacks.forget(id)
	}
	var data json.RawMessage
	if err := actions.Call(ctx, "session.finish", map[string]any{"session_id": id}, &data); err != nil {
		return false, err
	}
	if err := pluginwire.ValidateActionResult("session.finish", data); err != nil {
		return false, err
	}
	var result struct {
		Finished bool `json:"finished"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, err
	}
	return result.Finished, nil
}

// Ask registers a continuation, sends the prompt as a nonterminal action, and
// completes this event. Callbacks use the next request's EventContext. In a
// callback Ask reuses its session ID; Scope applies only to new conversations.
func (event *EventContext) Ask(ctx context.Context, prompt string, options SessionWaitOptions, next HandlerFunc) (SessionRef, error) {
	if event == nil || event.client == nil || next == nil {
		return SessionRef{}, errors.New("rayleabot: Ask requires an event and continuation")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if event.Event.EventType != "message.group" && event.Event.EventType != "message.private" {
		return SessionRef{}, errors.New("rayleabot: Ask requires a message event")
	}
	if event.Event.Session != nil && options.SessionID == "" {
		options.SessionID = event.Event.Session.SessionID
		options.Scope = ""
	}
	scope := options.Scope
	if event.Event.Session != nil && options.SessionID == event.Event.Session.SessionID {
		scope = event.Event.Session.Scope
	}
	if scope == "" || event.Event.Target.Type == "private" {
		scope = "user"
	}
	if err := event.client.callbacks.reserve(options.SessionID, sessionRoute(event, scope)); err != nil {
		return SessionRef{}, err
	}
	defer event.client.callbacks.release()
	ref, err := event.Actions().SessionWait(ctx, options)
	if err != nil {
		return SessionRef{}, err
	}
	cleanup := func(cause error) (SessionRef, error) {
		event.client.callbacks.forget(ref.SessionID)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, finishErr := event.Actions().SessionFinish(cleanupCtx, ref.SessionID)
		return SessionRef{}, errors.Join(cause, finishErr)
	}
	deadline := time.UnixMilli(ref.ExpiresAtMS)
	if callerDeadline, ok := ctx.Deadline(); ok && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	if err := event.client.callbacks.install(ref, sessionRoute(event, ref.Scope), deadline, next); err != nil {
		return cleanup(err)
	}
	_, err = event.Actions().MessageSend(ctx, MessageSendRequest{SourceAdapter: event.Event.SourceAdapter, SourceProtocol: event.Event.SourceProtocol, TargetType: event.Event.Target.Type, TargetID: event.Event.Target.ID, ReplyToEventID: event.Event.EventID, FallbackToSendIfMissing: true, Message: MessageOut{Segments: []Segment{Text(prompt)}}})
	if err != nil {
		return cleanup(err)
	}
	if err := event.Result(nil); err != nil {
		return cleanup(err)
	}
	return ref, nil
}

func sessionRoute(event *EventContext, scope string) string {
	actor := ""
	if scope == "user" {
		actor = event.Event.Actor.ID
	}
	encoded, _ := json.Marshal([7]string{event.Event.SourceProtocol, event.Event.SourceAdapter, event.Bot.ID, event.Event.Target.Type, event.Event.Target.ID, scope, actor})
	return string(encoded)
}
