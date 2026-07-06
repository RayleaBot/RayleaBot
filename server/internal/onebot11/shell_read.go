package onebot11

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func classifyFrame(messageType websocket.MessageType, payload []byte, observedAt time.Time) ClassifiedFrame {
	return ClassifyFrame(messageType, payload, observedAt)
}

func normalizeSupportedEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return NormalizeSupportedEvent(frame, observedAt)
}

func applyFrameSummary(snapshot *Snapshot, frame ClassifiedFrame) {
	if snapshot == nil {
		return
	}
	summary := frame.Summary

	snapshot.TotalReceivedFrames++
	snapshot.LastFrameCategory = summary.Category
	snapshot.LastFrameType = summary.Type
	if frame.Frame.SelfID > 0 {
		snapshot.BotID = fmt.Sprintf("%d", frame.Frame.SelfID)
	}

	if summary.Category == FrameCategoryInvalid {
		snapshot.InvalidReceivedFrames++
	} else {
		snapshot.LastFrameAt = cloneTime(&summary.ObservedAt)
	}

	if summary.Category == FrameCategoryHeartbeat {
		snapshot.HeartbeatSeen = true
		snapshot.LastHeartbeatAt = cloneTime(&summary.ObservedAt)
		if summary.HeartbeatInterval > 0 {
			snapshot.HeartbeatInterval = summary.HeartbeatInterval
		}
	}
}

func isReadySummary(summary FrameSummary) bool {
	return summary.Category == FrameCategoryLifecycleReady || summary.Category == FrameCategoryHeartbeat
}

func isLifecycleDisable(frame OneBotFrame) bool {
	return frame.PostType == "meta_event" && frame.MetaEventType == "lifecycle" && frame.SubType == "disable"
}

func (s *Shell) waitForReadyFrame(ctx context.Context, transport TransportKey, conn *websocket.Conn) (FrameSummary, error) {
	waitingForFirstFrame := true

	for {
		readyCtx, cancel := s.waitContext(ctx)
		frame, err := s.readFrame(readyCtx, conn)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				if waitingForFirstFrame {
					return FrameSummary{}, fmt.Errorf("timed out waiting for first frame: %w", err)
				}
				return FrameSummary{}, fmt.Errorf("timed out waiting for ready frame: %w", err)
			}
			return FrameSummary{}, err
		}

		if err := s.recordAndValidateFrame(transport, frame); err != nil {
			return FrameSummary{}, err
		}
		if isReadySummary(frame.Summary) {
			return frame.Summary, nil
		}

		waitingForFirstFrame = false
	}
}
func (s *Shell) readLoop(ctx context.Context, transport TransportKey, conn *websocket.Conn) error {
	for {
		readCtx, cancel := s.readContext(ctx)
		frame, err := s.readFrame(readCtx, conn)
		cancel()
		if err != nil {
			return err
		}

		if err := s.recordAndValidateFrame(transport, frame); err != nil {
			return err
		}

		s.routeAPIResponse(frame)
		s.forwardSupportedEvent(ctx, transport, frame)
	}
}

func (s *Shell) forwardSupportedEvent(ctx context.Context, transport TransportKey, frame ClassifiedFrame) {
	if frame.Summary.Category != FrameCategoryEvent {
		return
	}

	s.invalidateIdentityCacheForFrame(frame.Frame)

	normalizedEvent, ok := normalizeSupportedEvent(frame.Frame, frame.Summary.ObservedAt)
	if !ok {
		s.logger.Debug(
			"OneBot 事件未进入插件桥接：事件类型 "+frame.Summary.Type,
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"frame_type", frame.Summary.Type,
		)
		return
	}
	if s.isDuplicateEvent(normalizedEvent.EventID, frame.Summary.ObservedAt) {
		s.logger.Info(
			"OneBot 重复事件已丢弃：事件 ID "+normalizedEvent.EventID+"，类型 "+normalizedEvent.EventType,
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"error_code", errorCodeWebhookDuplicateEvent,
			"event_id", normalizedEvent.EventID,
			"event_type", normalizedEvent.EventType,
		)
		return
	}

	handler := s.currentEventHandler()
	if handler == nil {
		return
	}

	select {
	case s.eventQueue <- normalizedEvent:
	case <-ctx.Done():
		return
	default:
		s.logger.Warn(
			"OneBot 事件队列已满，已丢弃事件：类型 "+normalizedEvent.EventType,
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"event_kind", normalizedEvent.Kind,
			"event_type", normalizedEvent.EventType,
		)
	}
}

func (s *Shell) currentEventHandler() func(context.Context, NormalizedEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventHandler
}

func (s *Shell) currentReadyHandler() func(context.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readyHandler
}

func (s *Shell) dispatchEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventQueue:
			handler := s.currentEventHandler()
			if handler == nil {
				continue
			}
			handler(ctx, event)
		}
	}
}

func (s *Shell) readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	snapshot := s.Snapshot()
	timeout := s.provisionalReadTimeout(snapshot)
	return context.WithTimeout(ctx, timeout)
}
func (s *Shell) recordAndValidateFrame(transport TransportKey, frame ClassifiedFrame) error {
	snapshot := s.recordFrame(frame)

	switch {
	case isIgnoredAPIResponse(frame):
		s.logger.Warn(
			"OneBot API 响应没有匹配的请求，已忽略：端点 "+s.transportEndpoint(transport),
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"frame_type", frame.Summary.Type,
			"reason", frame.InvalidSummary,
			"echo_value_type", echoValueType(frame.Frame.Echo),
			"payload_preview", frame.PayloadPreview,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
		return nil
	case frame.Summary.Category == FrameCategoryInvalid:
		s.logger.Warn(
			"收到不合法的 OneBot 帧：端点 "+s.transportEndpoint(transport),
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"frame_type", frame.Summary.Type,
			"invalid_frame_count", snapshot.InvalidReceivedFrames,
			"reason", frame.InvalidSummary,
			"payload_preview", frame.PayloadPreview,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
		return fmt.Errorf("invalid frame: %s", frame.InvalidSummary)
	case isLifecycleDisable(frame.Frame):
		s.logger.Warn(
			"OneBot 上报生命周期 disable，适配器将按当前连接状态处理：端点 "+s.transportEndpoint(transport),
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
	}

	return nil
}
func isIgnoredAPIResponse(frame ClassifiedFrame) bool {
	return frame.Summary.Category == FrameCategoryUnknown && frame.Summary.Type == "api.response.ignored"
}
func echoValueType(value any) string {
	if value == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", value)
}
func (s *Shell) readFrame(ctx context.Context, conn *websocket.Conn) (ClassifiedFrame, error) {
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		return ClassifiedFrame{}, err
	}

	return classifyFrame(messageType, payload, s.deps.now()), nil
}
func (s *Shell) dial(ctx context.Context) (*websocket.Conn, *http.Response, error) {
	dialCtx, cancel := context.WithTimeout(ctx, s.deps.connectTimeout)
	defer cancel()

	headers := http.Header{}
	accessToken := strings.TrimSpace(s.cfg.ForwardWS.AccessToken)
	if accessToken != "" {
		headers.Set("Authorization", "Bearer "+accessToken)
	}

	return s.deps.dial(dialCtx, dialURL(s.forwardWSURL(), accessToken, s.cfg.ForwardWS.AccessTokenQueryCompat), &websocket.DialOptions{
		HTTPHeader: headers,
	})
}
