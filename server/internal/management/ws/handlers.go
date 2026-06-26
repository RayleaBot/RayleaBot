package ws

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

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
	ProtocolSnapshotEvent() managementevents.Frame
	SubscribeProtocolEvents(int) (<-chan managementevents.Frame, func())
}

type serviceStatusEventSource interface {
	CurrentEvent() managementevents.Frame
	Subscribe(int) (<-chan managementevents.Frame, func())
}

type governanceEventSource interface {
	Subscribe(int) (<-chan managementevents.Frame, func())
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

type TasksHandler struct {
	tasks taskEventSource
}

type taskEventSource interface {
	List() []tasks.Snapshot
	Subscribe(int) (<-chan tasks.Snapshot, func())
}

func NewTasksHandler(tasks taskEventSource) *TasksHandler {
	return &TasksHandler{tasks: tasks}
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
