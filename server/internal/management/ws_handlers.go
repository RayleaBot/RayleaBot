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

	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/console"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
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
			httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
			return
		}
		serveManagementWebSocket(w, r, h.streamEventsWebSocket)
	}
}

// serveManagementWebSocket upgrades the request, hands the connection to
// serve and closes it normally once serve returns. A failed upgrade has
// already written its response.
func serveManagementWebSocket(w http.ResponseWriter, r *http.Request, serve func(*websocket.Conn)) {
	conn, err := acceptManagementWebSocket(w, r)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()
	serve(conn)
}

// streamFrames writes the initial items, then forwards updates until the
// peer closes or the source ends. frame projects each item onto the wire.
func streamFrames[T any](ctx context.Context, conn *websocket.Conn, initial []T, updates <-chan T, frame func(T) managementevents.Frame) {
	for _, item := range initial {
		if err := wsjson.Write(ctx, conn, frame(item)); err != nil {
			return
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-updates:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, frame(item)); err != nil {
				return
			}
		}
	}
}

func (h *EventsHandler) streamEventsWebSocket(conn *websocket.Conn) {
	eventsCtx := conn.CloseRead(context.Background())
	_ = h.stream.Run(eventsCtx, func(ctx context.Context, value any) error { return wsjson.Write(ctx, conn, value) }, func() { _ = conn.CloseNow() })
}

func (h *LogsHandler) HandleLogsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
			return
		}
		serveManagementWebSocket(w, r, func(conn *websocket.Conn) {
			framesCtx := conn.CloseRead(context.Background())
			summaries, unsubscribe := h.logs.Subscribe(8)
			defer unsubscribe()
			streamFrames(framesCtx, conn, h.initialLogSummaries(framesCtx), summaries, newLogFrame)
		})
	}
}

// initialLogSummaries returns the persisted replay followed by the in-memory
// entries the replay did not already cover.
func (h *LogsHandler) initialLogSummaries(ctx context.Context) []logging.Summary {
	initial := h.logs.Replay(ctx)
	replayed := make(map[string]struct{}, len(initial))
	for _, summary := range initial {
		replayed[logSummaryKey(summary)] = struct{}{}
	}
	for _, summary := range h.logs.Snapshot() {
		if _, ok := replayed[logSummaryKey(summary)]; ok {
			continue
		}
		initial = append(initial, summary)
	}
	return initial
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

func newLogFrame(summary logging.Summary) managementevents.Frame {
	return managementevents.Frame{
		Channel:   "logs",
		Type:      "logs.appended",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      summary,
	}
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
			httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
			return
		}

		pluginID := strings.TrimSpace(chi.URLParam(r, "id"))
		if pluginID == "" {
			httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
			return
		}
		if _, ok := h.plugins.Get(pluginID); !ok {
			httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
			return
		}
		serveManagementWebSocket(w, r, func(conn *websocket.Conn) {
			framesCtx := conn.CloseRead(context.Background())
			entries, unsubscribe := h.console.Subscribe(pluginID, 8)
			defer unsubscribe()
			streamFrames(framesCtx, conn, h.console.Snapshot(pluginID), entries, newConsoleFrame)
		})
	}
}

func newConsoleFrame(entry console.Entry) managementevents.Frame {
	timestamp := entry.Timestamp.UTC().Format(time.RFC3339)
	return managementevents.Frame{
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
