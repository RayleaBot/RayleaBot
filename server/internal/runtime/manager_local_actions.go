package runtime

import (
	"context"
	"errors"
)

func isOneBotFamilyAction(kind string) bool {
	switch kind {
	case
		"message.get",
		"message.delete",
		"message.history.get",
		"message.forward.get",
		"message.forward.send",
		"message.read.mark",
		"friend.request.handle",
		"friend.list",
		"friend.remark.set",
		"user.info.get",
		"user.like.send",
		"group.list",
		"group.info.get",
		"group.member.get",
		"group.member.list",
		"group.request.handle",
		"group.leave",
		"group.admin.set",
		"group.ban.set",
		"group.card.set",
		"group.title.set",
		"group.name.set",
		"group.announcement.list",
		"group.announcement.create",
		"group.announcement.delete",
		"group.essence.list",
		"group.essence.set",
		"group.essence.unset",
		"group.honor.get",
		"group.todo.set",
		"file.get",
		"file.download",
		"file.group.upload",
		"file.private.upload",
		"file.group.url.get",
		"file.private.url.get",
		"file.group.fs.info",
		"file.group.fs.list",
		"file.group.fs.mkdir",
		"file.group.fs.delete",
		"reaction.set",
		"reaction.list",
		"poke.send":
		return true
	default:
		return false
	}
}

func isProviderExtensionAction(kind string) bool {
	switch kind {
	case
		"provider.napcat.message_emoji.like.set",
		"provider.napcat.group.sign.set",
		"provider.luckylillia.friend_groups.get":
		return true
	default:
		return false
	}
}

func (m *Manager) executeLocalAction(ctx context.Context, handle *processHandle, parentRequestID string, requestID string, action Action, parentEvent Event) {
	if m.opts.ExecuteLocalAction == nil {
		if err := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action executor is not available", nil); err != nil {
			m.failRuntime(handle, err.Code, err.Message, err.Err)
		}
		return
	}

	result, err := m.opts.ExecuteLocalAction(ctx, handle.spec.PluginID, requestID, action, parentEvent)
	if err != nil {
		var runtimeErr *Error
		if errors.As(err, &runtimeErr) {
			if writeErr := m.writeLocalError(handle, parentRequestID, requestID, runtimeErr.Code, runtimeErr.Message, runtimeErr.Details); writeErr != nil {
				m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
			}
			return
		}
		if writeErr := m.writeLocalError(handle, parentRequestID, requestID, codePluginInternalError, "plugin local action failed", nil); writeErr != nil {
			m.failRuntime(handle, writeErr.Code, writeErr.Message, writeErr.Err)
		}
		return
	}

	if result == nil {
		result = map[string]any{}
	}
	if err := m.writeLocalResult(handle, parentRequestID, requestID, result); err != nil {
		m.failRuntime(handle, err.Code, err.Message, err.Err)
	}
}

func (m *Manager) writeLocalResult(handle *processHandle, parentRequestID string, requestID string, data map[string]any) *Error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "result",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"status":           "success",
		"data":             data,
	}
	return m.writeLocalResponse(handle, parentRequestID, frame)
}

func (m *Manager) writeLocalError(handle *processHandle, parentRequestID string, requestID string, code string, message string, details map[string]any) *Error {
	frame := map[string]any{
		"protocol_version": "1",
		"type":             "error",
		"timestamp":        m.deps.now().Unix(),
		"plugin_id":        handle.spec.PluginID,
		"request_id":       requestID,
		"code":             code,
		"message":          message,
	}
	if len(details) > 0 {
		frame["details"] = cloneDetails(details)
	}
	return m.writeLocalResponse(handle, parentRequestID, frame)
}

func (m *Manager) writeLocalResponse(handle *processHandle, parentRequestID string, frame map[string]any) *Error {
	m.protocolMu.Lock()
	defer m.protocolMu.Unlock()

	m.mu.Lock()
	if m.proc != handle {
		m.mu.Unlock()
		return nil
	}
	session := m.pendingEvents[parentRequestID]
	if session == nil || session.completed {
		m.mu.Unlock()
		return nil
	}
	if session.pendingLocalAction > 0 {
		session.pendingLocalAction--
	}
	m.mu.Unlock()

	if err := handle.writeJSONLine(frame); err != nil {
		return errorf(codePluginInternalError, "write local action response frame", err)
	}
	return nil
}
