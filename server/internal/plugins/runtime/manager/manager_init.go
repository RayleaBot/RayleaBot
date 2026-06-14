package manager

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	runtimeprocess "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/process"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
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
			if status == runtimeprotocol.InitResponseReady {
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

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (runtimeprotocol.InitResponseStatus, []string, *Error) {
	var envelope runtimeprotocol.FrameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	if envelope.ProtocolVersion != "1" {
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unsupported protocol_version", nil)
	}
	if envelope.PluginID == "" || envelope.PluginID != pluginID {
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched plugin_id", nil)
	}
	if envelope.RequestID == "" || envelope.RequestID != requestID {
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned a mismatched request_id", nil)
	}

	switch envelope.Type {
	case "init_progress":
		var progress runtimeprotocol.InitProgressFrame
		if err := json.Unmarshal(line, &progress); err != nil {
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_progress", err)
		}

		summary := strings.TrimSpace(progress.Summary)
		if summary == "" {
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin init_progress is missing summary", nil)
		}
		return runtimeprotocol.InitResponseWait, []string{summary}, nil
	case "init_ack":
		var ack runtimeprotocol.InitAckFrame
		if err := json.Unmarshal(line, &ack); err != nil {
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed init_ack", err)
		}
		if ack.Status == "ready" {
			return runtimeprotocol.InitResponseReady, append([]string(nil), ack.Subscriptions...), nil
		}
		if ack.Status == "error" {
			message := strings.TrimSpace(ack.ErrorMessage)
			if message == "" {
				message = "plugin reported init error"
			}
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginInternalError, message, nil)
		}
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned unsupported init_ack status", nil)
	case "error":
		var frame runtimeprotocol.ErrorFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed error frame", err)
		}
		if frame.Code == "" || frame.Message == "" {
			return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin error frame is missing code or message", nil)
		}
		return runtimeprotocol.InitResponseWait, nil, errorWithDetails(frame.Code, frame.Message, frame.Details, nil)
	default:
		return runtimeprotocol.InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an unexpected protocol message during init", nil)
	}
}
