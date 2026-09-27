package runtime

import (
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

// Defaults of runtime.plugin_detached_event_timeout_seconds and
// runtime.max_detached_events_per_plugin, used when no configuration is wired.
const (
	defaultDetachedEventTimeoutSeconds = 900
	defaultMaxDetachedEvents           = 8
)

// detachedSession is the background state of an event after event.detach. The
// event keeps its request ID, origin and session context until it ends.
type detachedSession struct {
	pluginID string
	delivery plugins.Delivery
	timer    *time.Timer
}

func detachableEvent(eventType string) bool {
	switch eventType {
	case "message.private", "message.group", "scheduler.trigger", "management.action":
		return true
	default:
		return false
	}
}

func isMessageEvent(eventType string) bool {
	return eventType == "message.private" || eventType == "message.group"
}

// detachEventLocked moves a foreground event to the background: the delivery
// completes with the plugin's result while the session keeps accepting actions
// until its terminal, its background deadline or the runtime stop.
func (m *Manager) detachEventLocked(handle *Handle, session *eventSession, requestID string, action plugins.Action) localActionReply {
	reply := localActionReply{parentRequestID: session.requestID, requestID: requestID}
	reject := func(code, message string, details map[string]any) localActionReply {
		reply.code, reply.message, reply.details = code, message, details
		return reply
	}
	switch {
	case !detachableEvent(session.event.EventType):
		return reject(codePlatformInvalidRequest, "this event type cannot move to the background", nil)
	case session.detached != nil:
		return reject(codePlatformInvalidRequest, "the event is already in the background", nil)
	case action.DetachPropagation != "" && !isMessageEvent(session.event.EventType):
		return reject(codePlatformInvalidRequest, "only message events can decide propagation", nil)
	}
	limits := m.opts.RuntimeConfig()
	limit := positiveInt(limits.MaxDetachedEventsPerPlugin, defaultMaxDetachedEvents)
	active := 0
	for _, pending := range m.pendingEvents {
		if pending.detached != nil {
			active++
		}
	}
	if active >= limit {
		return reject(codePlatformRateLimited, "background event limit reached", map[string]any{"limit": limit})
	}

	timeout := durationFromSeconds(limits.PluginDetachedEventTimeoutSeconds, defaultDetachedEventTimeoutSeconds)
	now := time.Now()
	result := action.DetachResult
	if result == nil {
		result = map[string]any{}
	}
	session.deadline = now.Add(timeout)
	session.detached = &detachedSession{
		pluginID: handle.Spec.PluginID,
		delivery: plugins.Delivery{
			RequestID:   session.requestID,
			Result:      result,
			Propagation: action.DetachPropagation,
			Detached:    plugins.NewDetachedEvent(),
		},
		timer: time.AfterFunc(timeout, func() { m.expireDetached(handle, session) }),
	}
	close(session.detachedCh)
	m.logger.Info("插件"+pluginIDLabel(handle.Spec.PluginID)+"的事件已转入后台", detachedLogAttrs(session, now)...)
	reply.result = map[string]any{"deadline_at_ms": session.deadline.UnixMilli()}
	return reply
}

// expireDetached ends a background event at its deadline. Its pending service
// calls fail now; later actions and a late terminal match an expired event.
func (m *Manager) expireDetached(handle *Handle, session *eventSession) {
	failure := errorf(codePluginEventTimeout, "插件后台事件超过期限", nil)
	var canceledCalls []string
	m.mu.Lock()
	if m.proc != handle || session.completed || m.pendingEvents[session.requestID] != session {
		m.mu.Unlock()
		return
	}
	for requestID := range session.serviceCancels {
		canceledCalls = append(canceledCalls, requestID)
	}
	m.completeEventLocked(session, failedDelivery(session.requestID, failure), failure)
	m.markEventExpiredLocked(session.requestID)
	m.mu.Unlock()
	for _, requestID := range canceledCalls {
		_ = handle.WriteJSONLine(localErrorFrame(requestID, failure.Code, failure.Message, nil))
	}
}

// cancelDetachedLocked ends background events when the runtime stops or is
// replaced by a reload. The stopping process is not told; its later frames for
// these events are ignored.
func (m *Manager) cancelDetachedLocked() {
	for _, session := range m.pendingEvents {
		if session.detached == nil || session.completed {
			continue
		}
		failure := errorf(codePluginEventCanceled, "插件停止，后台事件已取消", nil)
		m.completeEventLocked(session, failedDelivery(session.requestID, failure), failure)
		m.markEventExpiredLocked(session.requestID)
	}
}

// endDetachedLocked records the end of a background event, a timeout as a
// warning, and then reports it to the dispatcher that released the event.
func (m *Manager) endDetachedLocked(session *eventSession, err error) {
	detached := session.detached
	detached.timer.Stop()
	code := ""
	var runtimeErr *plugins.Error
	if errors.As(err, &runtimeErr) {
		code = runtimeErr.Code
	} else if err != nil {
		code = errorcodes.PluginInternalError
	}
	label := "插件" + pluginIDLabel(detached.pluginID)
	attrs := detachedLogAttrs(session, time.Now())
	switch code {
	case "":
		m.logger.Info(label+"的后台事件已结束", attrs...)
	case codePluginEventTimeout:
		// The runtime owns this failure record; dispatch does not repeat it.
		runtimeErr.MarkFailureReported()
		m.logger.Warn(label+"的后台事件超过期限，已结束并取消其未完成的服务调用", append(attrs, "error_code", code)...)
	default:
		m.logger.Info(label+"的后台事件已结束", append(attrs, "error_code", code)...)
	}
	detached.delivery.Detached.Finish(err)
}

// detachedLogAttrs describes a background event without message content.
func detachedLogAttrs(session *eventSession, now time.Time) []any {
	attrs := []any{
		"component", "runtime",
		"plugin_id", session.detached.pluginID,
		"event_type", session.event.EventType,
		"event_id", session.event.EventID,
		"source_protocol", session.event.SourceProtocol,
		"source_adapter", session.event.SourceAdapter,
		"deadline_at_ms", session.deadline.UnixMilli(),
		"duration_ms", now.Sub(session.startedAt).Milliseconds(),
	}
	if taskID, ok := session.event.PayloadFields["task_id"].(string); ok && session.event.EventType == "scheduler.trigger" {
		attrs = append(attrs, "task_id", taskID)
	}
	return attrs
}

func failedDelivery(requestID string, failure *plugins.Error) plugins.Delivery {
	return plugins.Delivery{RequestID: requestID, ErrorCode: failure.Code, ErrorMessage: failure.Message, ErrorDetails: cloneDetails(failure.Details)}
}
