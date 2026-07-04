package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (m *Manager) awaitInitAck(ctx context.Context, handle *Handle, requestID string) ([]string, *Error) {
	silenceTimer := time.NewTimer(handle.Spec.InitTimeout)
	defer silenceTimer.Stop()

	totalTimer := time.NewTimer(handle.Spec.InitMaxTotal)
	defer totalTimer.Stop()

	for {
		readCh := make(chan []byte, 1)
		readErrCh := make(chan error, 1)

		go func() {
			line, err := handle.Stdout.ReadBytes('\n')
			if err != nil {
				readErrCh <- err
				return
			}
			readCh <- line
		}()

		select {
		case line := <-readCh:
			status, payload, err := m.parseInitResponse(line, handle.Spec.PluginID, requestID)
			if err != nil {
				return nil, err
			}
			if status == InitResponseReady {
				return payload, nil
			}
			summary := ""
			if len(payload) > 0 {
				summary = payload[0]
			}

			m.logger.Info(
				"插件"+pluginIDLabel(handle.Spec.PluginID)+"运行时初始化中："+summary,
				"component", "runtime",
				"plugin_id", handle.Spec.PluginID,
				"runtime_state", string(StateStarting),
				"summary", summary,
			)
			ResetTimer(silenceTimer, handle.Spec.InitTimeout)
		case readErr := <-readErrCh:
			return nil, classifyProtocolReadError(handle, readErr, "plugin exited before init_ack", "read plugin init response")
		case <-handle.Done():
			waitErr, _ := handle.ExitResult()
			if waitErr == nil {
				return nil, errorf(codePluginInternalError, "plugin exited before init_ack", nil)
			}
			return nil, errorf(codePluginInternalError, "plugin exited before init_ack", waitErr)
		case <-silenceTimer.C:
			return nil, errorf(codePluginInitTimeout, "plugin init_ack timed out", nil)
		case <-totalTimer.C:
			return nil, errorf(codePluginInitTimeout, "plugin init exceeded maximum total duration", nil)
		case <-ctx.Done():
			return nil, errorf(codePluginInitTimeout, "plugin init_ack timed out", ctx.Err())
		}
	}
}

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (InitResponseStatus, []string, *Error) {
	var envelope FrameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	if envelope.ProtocolVersion != "1" {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "init_progress":
		var progress InitProgressFrame
		if err := json.Unmarshal(line, &progress); err != nil {
			return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_progress", err)
		}

		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin init_progress is missing summary", nil)
		}
		return InitResponseWait, []string{summary}, nil
	case "init_ack":
		var ack InitAckFrame
		if err := json.Unmarshal(line, &ack); err != nil {
			return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_ack", err)
		}
		if ack.Status == "ready" {
			return InitResponseReady, append([]string(nil), ack.Subscriptions...), nil
		}
		if ack.Status == "error" {
			message := strings.TrimSpace(ack.ErrorMessage)
			if message == "" {
				message = "plugin reported init error"
			}
			return InitResponseWait, nil, errorf(codePluginInternalError, message, nil)
		}
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned unsupported init_ack status", nil)
	case "error":
		var frame ErrorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		return InitResponseWait, nil, errorWithDetails(frame.Code, frame.Message, frame.Details, nil)
	default:
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during init", nil)
	}
}
