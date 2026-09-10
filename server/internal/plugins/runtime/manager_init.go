package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const pluginExitedBeforeInitMessage = "插件进程在初始化完成前退出，请查看该插件的 stderr 日志"

func (m *Manager) awaitInitAck(ctx context.Context, handle *Handle, requestID string) *plugins.Error {
	deadlineTimer := time.NewTimer(handle.Spec.InitTimeout)
	defer deadlineTimer.Stop()

	for {
		readCh := make(chan []byte, 1)
		readErrCh := make(chan error, 1)

		go func() {
			line, err := readProtocolLine(handle.Stdout, handle.Spec.IPCMessageMaxBytes)
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
				return err
			}
			if status == InitResponseReady {
				return nil
			}
			summary := ""
			if len(payload) > 0 {
				summary = payload[0]
			}

			m.logger.Debug(
				"插件启动进度",
				"component", "runtime",
				"plugin_id", handle.Spec.PluginID,
				"runtime_state", string(StateStarting),
				"summary", summary,
			)
		case readErr := <-readErrCh:
			return classifyProtocolReadError(handle, readErr, pluginExitedBeforeInitMessage, "read plugin init response")
		case <-handle.Done():
			waitErr, _ := handle.ExitResult()
			if waitErr == nil {
				return errorf(codePluginInternalError, pluginExitedBeforeInitMessage, nil)
			}
			return errorf(codePluginInternalError, pluginExitedBeforeInitMessage, waitErr)
		case <-deadlineTimer.C:
			return errorf(codePluginInitTimeout, "plugin init_ack timed out", nil)
		case <-ctx.Done():
			return errorf(codePluginInitTimeout, "plugin init_ack timed out", ctx.Err())
		}
	}
}

func (m *Manager) parseInitResponse(line []byte, pluginID string, requestID string) (InitResponseStatus, []string, *plugins.Error) {
	if err := validatePluginFrame(line); err != nil {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned an invalid init response", err)
	}
	var envelope FrameEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return InitResponseWait, nil, errorf(codePluginProtocolViolation, "plugin returned malformed protocol json", err)
	}

	_ = pluginID
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
			return InitResponseReady, nil, nil
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
