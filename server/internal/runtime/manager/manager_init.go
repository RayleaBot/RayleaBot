package manager

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/runtime/process"
)

func (m *Manager) awaitInitAck(ctx context.Context, handle *runtimeprocess.Handle, requestID string) ([]string, *Error) {
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
			if status == initResponseReady {
				return payload, nil
			}
			summary := ""
			if len(payload) > 0 {
				summary = payload[0]
			}

			m.logger.Info(
				"plugin runtime init progress",
				"component", "runtime",
				"plugin_id", handle.Spec.PluginID,
				"runtime_state", string(StateStarting),
				"summary", summary,
			)
			runtimeprocess.ResetTimer(silenceTimer, handle.Spec.InitTimeout)
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

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (initResponseStatus, []string, *Error) {
	var envelope frameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	if envelope.ProtocolVersion != "1" {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "init_progress":
		var progress initProgressFrame
		if err := json.Unmarshal(line, &progress); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_progress", err)
		}

		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin init_progress is missing summary", nil)
		}
		return initResponseWait, []string{summary}, nil
	case "init_ack":
		var ack initAckFrame
		if err := json.Unmarshal(line, &ack); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_ack", err)
		}
		if ack.Status == "ready" {
			return initResponseReady, append([]string(nil), ack.Subscriptions...), nil
		}
		if ack.Status == "error" {
			message := strings.TrimSpace(ack.ErrorMessage)
			if message == "" {
				message = "plugin reported init error"
			}
			return initResponseWait, nil, errorf(codePluginInternalError, message, nil)
		}
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned unsupported init_ack status", nil)
	case "error":
		var frame errorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		return initResponseWait, nil, errorWithDetails(frame.Code, frame.Message, frame.Details, nil)
	default:
		return initResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during init", nil)
	}
}
