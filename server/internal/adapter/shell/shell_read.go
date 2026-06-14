package shell

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/coder/websocket"
)

func (s *Shell) waitForReadyFrame(ctx context.Context, transport TransportKey, conn *websocket.Conn) (adapterintake.FrameSummary, error) {
	waitingForFirstFrame := true

	for {
		readyCtx, cancel := s.waitContext(ctx)
		frame, err := s.readFrame(readyCtx, conn)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				if waitingForFirstFrame {
					return adapterintake.FrameSummary{}, fmt.Errorf("timed out waiting for first frame: %w", err)
				}
				return adapterintake.FrameSummary{}, fmt.Errorf("timed out waiting for ready frame: %w", err)
			}
			return adapterintake.FrameSummary{}, err
		}

		if err := s.recordAndValidateFrame(transport, frame); err != nil {
			return adapterintake.FrameSummary{}, err
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
func (s *Shell) readContext(ctx context.Context) (context.Context, context.CancelFunc) {
	snapshot := s.Snapshot()
	timeout := s.provisionalReadTimeout(snapshot)
	return context.WithTimeout(ctx, timeout)
}
func (s *Shell) recordAndValidateFrame(transport TransportKey, frame adapterintake.ClassifiedFrame) error {
	snapshot := s.recordFrame(frame)

	switch {
	case isIgnoredAPIResponse(frame):
		s.logger.Warn(
			"ignored OneBot API response with unsupported echo",
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
	case frame.Summary.Category == adapterintake.FrameCategoryInvalid:
		s.logger.Warn(
			"invalid OneBot frame received",
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
			"adapter lifecycle disable observed",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"frame_type", frame.Summary.Type,
			"transport", string(transport),
			"endpoint", s.transportEndpoint(transport),
		)
	}

	return nil
}
func isIgnoredAPIResponse(frame adapterintake.ClassifiedFrame) bool {
	return frame.Summary.Category == adapterintake.FrameCategoryUnknown && frame.Summary.Type == "api.response.ignored"
}
func echoValueType(value any) string {
	if value == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", value)
}
func (s *Shell) readFrame(ctx context.Context, conn *websocket.Conn) (adapterintake.ClassifiedFrame, error) {
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		return adapterintake.ClassifiedFrame{}, err
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

	return s.deps.dial(dialCtx, dialURL(s.forwardWSURL(), accessToken), &websocket.DialOptions{
		HTTPHeader: headers,
	})
}
