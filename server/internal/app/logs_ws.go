package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type logFrame struct {
	Channel   string          `json:"channel"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      logging.Summary `json:"data"`
}

func (h *logsWSHandler) handleLogsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		framesCtx := conn.CloseRead(context.Background())
		summaries, unsubscribe := h.logs.stream.Subscribe(8)
		defer unsubscribe()

		replayed := make(map[string]struct{})
		for _, summary := range h.logs.replayLogSummaries(framesCtx) {
			if err := wsjson.Write(framesCtx, conn, newLogFrame(summary)); err != nil {
				return
			}
			replayed[logSummaryKey(summary)] = struct{}{}
		}

		for _, summary := range h.logs.stream.Snapshot() {
			if _, ok := replayed[logSummaryKey(summary)]; ok {
				continue
			}
			if err := wsjson.Write(framesCtx, conn, newLogFrame(summary)); err != nil {
				return
			}
		}

		for {
			select {
			case <-framesCtx.Done():
				return
			case summary, ok := <-summaries:
				if !ok {
					return
				}
				if err := wsjson.Write(framesCtx, conn, newLogFrame(summary)); err != nil {
					return
				}
			}
		}
	}
}

func (s *logService) replayLogSummaries(ctx context.Context) []logging.Summary {
	if s == nil {
		return nil
	}
	limit := 32
	if s.stream != nil && s.stream.Limit() > 0 {
		limit = s.stream.Limit()
	}
	items, err := s.listLogSummaries(ctx, logging.Query{Limit: limit})
	if err != nil {
		return nil
	}
	return items
}

func logSummaryKey(summary logging.Summary) string {
	if summary.LogID != "" {
		return summary.LogID
	}

	return strings.Join([]string{
		summary.LogID,
		summary.Timestamp,
		summary.Level,
		summary.Source,
		summary.Message,
		summary.PluginID,
		summary.RequestID,
	}, "\x1f")
}

func newLogFrame(summary logging.Summary) logFrame {
	return logFrame{
		Channel:   "logs",
		Type:      "logs.appended",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      summary,
	}
}
