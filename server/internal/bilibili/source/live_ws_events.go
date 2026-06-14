package source

import (
	"context"
	"strings"
)

func (s *Source) handleLiveWebSocketEvent(ctx context.Context, subject Subject, roomID string, event map[string]any) {
	cmd := strings.TrimSpace(stringValue(event["cmd"]))
	if strings.Contains(cmd, ":") {
		cmd = strings.SplitN(cmd, ":", 2)[0]
	}
	switch cmd {
	case "LIVE":
		items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
		if err != nil {
			s.emitSyntheticLiveTransition(ctx, subject, roomID, 1)
			return
		}
		if item, ok := items[subject.UID]; ok {
			s.emitLiveTransition(ctx, subject, item, 1, "websocket")
		}
	case "PREPARING":
		items, err := s.fetchLiveStatuses(ctx, []Subject{subject})
		if err != nil {
			s.emitSyntheticLiveTransition(ctx, subject, roomID, 0)
			return
		}
		if item, ok := items[subject.UID]; ok {
			s.emitLiveTransition(ctx, subject, item, 0, "websocket")
		}
	}
}
