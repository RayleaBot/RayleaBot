package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type eventsWSHandler struct {
	bridge        eventBridgeSource
	plugins       pluginEventSource
	protocol      protocolEventSource
	serviceStatus serviceStatusEventSource
	governance    governanceEventSource
	bilibili      bilibiliEventSource
}

type eventBridgeSource interface {
	SubscribeObservability(int) (<-chan bridge.ObservabilityFrame, func())
}

type pluginEventSource interface {
	Subscribe(int) (<-chan plugins.Snapshot, func())
	List() []plugins.Snapshot
}

type protocolEventSource interface {
	protocolSnapshotEvent() managementEventFrame
	subscribeProtocolEvents(int) (<-chan managementEventFrame, func())
}

type serviceStatusEventSource interface {
	currentServiceStatusEvent() managementEventFrame
	subscribeStatusEvents(int) (<-chan managementEventFrame, func())
}

type governanceEventSource interface {
	subscribeGovernanceEvents(int) (<-chan managementEventFrame, func())
}

type bilibiliEventSource interface {
	currentEvent() managementEventFrame
	subscribe(int) (<-chan managementEventFrame, func())
}

func newEventsWSHandler(bridge eventBridgeSource, plugins pluginEventSource, protocol protocolEventSource, serviceStatus serviceStatusEventSource, governance governanceEventSource, bilibili bilibiliEventSource) *eventsWSHandler {
	return &eventsWSHandler{bridge: bridge, plugins: plugins, protocol: protocol, serviceStatus: serviceStatus, governance: governance, bilibili: bilibili}
}

type tasksWSHandler struct {
	tasks taskEventSource
}

type taskEventSource interface {
	List() []tasks.Snapshot
	Subscribe(int) (<-chan tasks.Snapshot, func())
}

func newTasksWSHandler(tasks taskEventSource) *tasksWSHandler {
	return &tasksWSHandler{tasks: tasks}
}

type logsWSHandler struct {
	logs logEventSource
}

type logEventSource interface {
	Replay(context.Context) []logging.Summary
	Snapshot() []logging.Summary
	Subscribe(int) (<-chan logging.Summary, func())
}

func newLogsWSHandler(logs logEventSource) *logsWSHandler {
	return &logsWSHandler{logs: logs}
}

type consoleWSHandler struct {
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

func newConsoleWSHandler(console consoleEventSource, plugins pluginLookupSource) *consoleWSHandler {
	return &consoleWSHandler{console: console, plugins: plugins}
}
