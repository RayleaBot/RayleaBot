package management

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type webSocketOriginAuthorityKey struct{}
type webSocketCredentialsChangedKey struct{}

func acceptManagementWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	authority, ok := r.Context().Value(webSocketOriginAuthorityKey{}).(string)
	if !ok || strings.TrimSpace(authority) == "" {
		return nil, errors.New("validated websocket origin is required")
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{authority}})
	if err != nil {
		return nil, err
	}
	if changed, ok := r.Context().Value(webSocketCredentialsChangedKey{}).(<-chan struct{}); ok {
		channel := "events"
		if r.URL.Path == "/ws/logs" {
			channel = "logs"
		} else if strings.HasPrefix(r.URL.Path, "/ws/plugins/") {
			channel = "plugin_console"
		}
		go func() {
			select {
			case <-r.Context().Done():
				return
			case <-changed:
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = wsjson.Write(ctx, conn, managementevents.Frame{Channel: channel, Type: "session_expired", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Data: struct{}{}})
				_ = conn.Close(websocket.StatusPolicyViolation, "session invalidated")
			}
		}()
	}
	return conn, nil
}

func writeWebSocketPermissionDenied(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,

		errorcodes.PermissionDenied,

		nil,
	)
}

func writeWebSocketNotFound(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,

		errorcodes.PlatformResourceNotFound,

		nil,
	)
}

type EventsHandler struct{ stream *managementevents.Stream }

func NewEventsHandler(sources managementevents.Sources) (*EventsHandler, error) {
	stream, err := managementevents.NewStream(sources)
	if err != nil {
		return nil, err
	}
	return &EventsHandler{stream: stream}, nil
}

func (h *EventsHandler) Close() { h.stream.Close() }

type LogsHandler struct {
	logs logEventSource
}

type logEventSource interface {
	Replay(context.Context) []logging.Summary
	Snapshot() []logging.Summary
	Subscribe(int) (<-chan logging.Summary, func())
}

func NewLogsHandler(logs logEventSource) *LogsHandler {
	return &LogsHandler{logs: logs}
}

type ConsoleHandler struct {
	console consoleEventSource
	plugins pluginLookupSource
}

type consoleEventSource interface {
	Snapshot(string) []console.Entry
	Subscribe(string, int) (<-chan console.Entry, func())
}

type pluginLookupSource interface {
	Get(string) (plugins.Snapshot, bool)
}

func NewConsoleHandler(console consoleEventSource, plugins pluginLookupSource) *ConsoleHandler {
	return &ConsoleHandler{console: console, plugins: plugins}
}

func (h *EventsHandler) HandleEventsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			writeWebSocketPermissionDenied(w, r)
			return
		}

		conn, err := acceptManagementWebSocket(w, r)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		h.streamEventsWebSocket(conn)
	}
}

func (h *EventsHandler) streamEventsWebSocket(conn *websocket.Conn) {
	eventsCtx := conn.CloseRead(context.Background())
	_ = h.stream.Run(eventsCtx, func(ctx context.Context, value any) error { return wsjson.Write(ctx, conn, value) }, func() { _ = conn.CloseNow() })
}

type logFrame struct {
	Channel   string          `json:"channel"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      logging.Summary `json:"data"`
}

func (h *LogsHandler) HandleLogsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			writeWebSocketPermissionDenied(w, r)
			return
		}

		conn, err := acceptManagementWebSocket(w, r)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		framesCtx := conn.CloseRead(context.Background())
		summaries, unsubscribe := h.logs.Subscribe(8)
		defer unsubscribe()

		replayed := make(map[string]struct{})
		for _, summary := range h.logs.Replay(framesCtx) {
			if err := wsjson.Write(framesCtx, conn, newLogFrame(summary)); err != nil {
				return
			}
			replayed[logSummaryKey(summary)] = struct{}{}
		}

		for _, summary := range h.logs.Snapshot() {
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

type consoleFrame struct {
	Channel   string           `json:"channel"`
	Type      string           `json:"type"`
	Timestamp string           `json:"timestamp"`
	Data      consoleFrameData `json:"data"`
}

type consoleFrameData struct {
	PluginID  string `json:"plugin_id"`
	Stream    string `json:"stream"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

func (h *ConsoleHandler) HandlePluginConsoleWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			writeWebSocketPermissionDenied(w, r)
			return
		}

		pluginID := strings.TrimSpace(chi.URLParam(r, "id"))
		if pluginID == "" {
			writeWebSocketNotFound(w, r)
			return
		}
		if _, ok := h.plugins.Get(pluginID); !ok {
			writeWebSocketNotFound(w, r)
			return
		}

		conn, err := acceptManagementWebSocket(w, r)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		framesCtx := conn.CloseRead(context.Background())
		entries, unsubscribe := h.console.Subscribe(pluginID, 8)
		defer unsubscribe()

		for _, entry := range h.console.Snapshot(pluginID) {
			if err := wsjson.Write(framesCtx, conn, newConsoleFrame(entry)); err != nil {
				return
			}
		}

		for {
			select {
			case <-framesCtx.Done():
				return
			case entry, ok := <-entries:
				if !ok {
					return
				}
				if err := wsjson.Write(framesCtx, conn, newConsoleFrame(entry)); err != nil {
					return
				}
			}
		}
	}
}

func newConsoleFrame(entry console.Entry) consoleFrame {
	timestamp := entry.Timestamp.UTC().Format(time.RFC3339)
	return consoleFrame{
		Channel:   "plugin_console",
		Type:      "plugins.console",
		Timestamp: timestamp,
		Data: consoleFrameData{
			PluginID:  entry.PluginID,
			Stream:    entry.Stream,
			Text:      entry.Text,
			Timestamp: timestamp,
		},
	}
}
