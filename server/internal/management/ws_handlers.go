package management

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

var managementWebSocketAcceptOptions = &websocket.AcceptOptions{
	OriginPatterns: []string{
		"127.0.0.1:4173",
		"localhost:4173",
	},
}

func acceptManagementWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, managementWebSocketAcceptOptions)
}

func writeWebSocketPermissionDenied(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusUnauthorized,
		"permission.denied",
		"当前用户无权执行该操作",
		"errors.permission.denied",
		nil,
	)
}

func writeWebSocketNotFound(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusNotFound,
		"platform.resource_missing",
		"缺少必要资源",
		"errors.platform.resource_missing",
		nil,
	)
}

type EventsHandler struct {
	bridge        eventBridgeSource
	plugins       pluginEventSource
	protocol      protocolEventSource
	serviceStatus serviceStatusEventSource
	governance    governanceEventSource
}

type eventBridgeSource interface {
	SubscribeObservability(int) (<-chan bridge.ObservabilityFrame, func())
}

type pluginEventSource interface {
	Subscribe(int) (<-chan plugins.Snapshot, func())
	List() []plugins.Snapshot
}

type protocolEventSource interface {
	ProtocolSnapshotEvent() Frame
	SubscribeProtocolEvents(int) (<-chan Frame, func())
}

type serviceStatusEventSource interface {
	CurrentEvent() Frame
	Subscribe(int) (<-chan Frame, func())
}

type governanceEventSource interface {
	Subscribe(int) (<-chan Frame, func())
}

func NewEventsHandler(bridge eventBridgeSource, plugins pluginEventSource, protocol protocolEventSource, serviceStatus serviceStatusEventSource, governance governanceEventSource) *EventsHandler {
	return &EventsHandler{bridge: bridge, plugins: plugins, protocol: protocol, serviceStatus: serviceStatus, governance: governance}
}

func (h *EventsHandler) SetBridge(bridge eventBridgeSource) {
	if h == nil {
		return
	}
	h.bridge = bridge
}

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
	bridgeFrames, unsubscribeBridge := h.bridge.SubscribeObservability(1)
	defer unsubscribeBridge()
	var pluginFrames <-chan plugins.Snapshot
	unsubscribePlugins := func() {}
	if h.plugins != nil {
		pluginFrames, unsubscribePlugins = h.plugins.Subscribe(8)
	}
	defer unsubscribePlugins()
	protocolFrames, unsubscribeProtocol := h.protocol.SubscribeProtocolEvents(2)
	defer unsubscribeProtocol()
	statusFrames, unsubscribeStatus := h.serviceStatus.Subscribe(4)
	defer unsubscribeStatus()
	var governanceFrames <-chan Frame
	unsubscribeGovernance := func() {}
	if h.governance != nil {
		governanceFrames, unsubscribeGovernance = h.governance.Subscribe(4)
	}
	defer unsubscribeGovernance()

	for _, frame := range []Frame{
		h.serviceStatus.CurrentEvent(),
		h.protocol.ProtocolSnapshotEvent(),
	} {
		if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
			return
		}
	}

	for {
		select {
		case <-eventsCtx.Done():
			return
		case frame, ok := <-bridgeFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case snapshot, ok := <-pluginFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, pluginStateEventFrame(snapshot, pluginSnapshotsForConflicts(h.plugins))); err != nil {
				return
			}
		case frame, ok := <-protocolFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case frame, ok := <-statusFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		case frame, ok := <-governanceFrames:
			if !ok {
				return
			}
			if err := wsjson.Write(eventsCtx, conn, frame); err != nil {
				return
			}
		}
	}
}

func pluginStateEventFrame(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) Frame {
	state, diagnosis := plugins.ProjectState(snapshot)
	return NewReceivedFrame(PluginStatePayload{
		PluginID:         snapshot.PluginID,
		State:            state,
		StateDiagnosis:   diagnosis,
		Commands:         pluginStateEventCommands(snapshot.Commands),
		CommandConflicts: pluginStateEventCommandConflicts(snapshot, snapshots),
	})
}

func pluginSnapshotsForConflicts(catalog interface{ List() []plugins.Snapshot }) []plugins.Snapshot {
	if catalog == nil {
		return nil
	}
	return catalog.List()
}

func pluginStateEventCommands(commands []plugins.Command) []PluginCommandItem {
	if len(commands) == 0 {
		return []PluginCommandItem{}
	}
	items := make([]PluginCommandItem, 0, len(commands))
	for _, command := range commands {
		if command.Name == "" {
			continue
		}
		item := PluginCommandItem{
			Name:          command.Name,
			Aliases:       append([]string(nil), command.Aliases...),
			Description:   command.Description,
			Usage:         command.Usage,
			Permission:    command.Permission,
			CommandSource: pluginEventCommandSource(command.CommandSource),
			DeclarationID: command.DeclarationID,
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return []PluginCommandItem{}
	}
	return items
}

func pluginStateEventCommandConflicts(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) []string {
	if len(snapshots) == 0 {
		snapshots = []plugins.Snapshot{snapshot}
	}
	conflicts := plugins.DetectCommandConflicts(snapshots)
	if len(conflicts[snapshot.PluginID]) == 0 {
		return []string{}
	}
	return conflicts[snapshot.PluginID]
}

func pluginEventCommandSource(source string) string {
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	if source == plugins.CommandSourcePattern {
		return plugins.CommandSourcePattern
	}
	return plugins.CommandSourceManifest
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
