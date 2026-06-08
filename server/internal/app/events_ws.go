package app

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (h *eventsWSHandler) handleEventsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		conn, err := acceptManagementWebSocket(w, r)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		eventsCtx := conn.CloseRead(context.Background())
		bridgeFrames, unsubscribeBridge := h.bridge.SubscribeObservability(1)
		defer unsubscribeBridge()
		var pluginFrames <-chan plugins.Snapshot
		unsubscribePlugins := func() {}
		if h.plugins != nil {
			pluginFrames, unsubscribePlugins = h.plugins.Subscribe(8)
		}
		defer unsubscribePlugins()
		protocolFrames, unsubscribeProtocol := h.protocol.subscribeProtocolEvents(2)
		defer unsubscribeProtocol()
		statusFrames, unsubscribeStatus := h.serviceStatus.subscribeStatusEvents(4)
		defer unsubscribeStatus()
		var bilibiliFrames <-chan managementEventFrame
		unsubscribeBilibili := func() {}
		if h.bilibili != nil {
			bilibiliFrames, unsubscribeBilibili = h.bilibili.subscribe(4)
		}
		defer unsubscribeBilibili()
		var governanceFrames <-chan managementEventFrame
		unsubscribeGovernance := func() {}
		if h.governance != nil {
			governanceFrames, unsubscribeGovernance = h.governance.subscribeGovernanceEvents(4)
		}
		defer unsubscribeGovernance()

		for _, frame := range []managementEventFrame{
			h.serviceStatus.currentServiceStatusEvent(),
			h.protocol.protocolSnapshotEvent(),
			h.bilibili.currentEvent(),
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
			case frame, ok := <-bilibiliFrames:
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
}

func pluginStateEventFrame(snapshot plugins.Snapshot, snapshots []plugins.Snapshot) managementEventFrame {
	return newEventsReceivedFrame(pluginStateEventPayload{
		PluginID:          snapshot.PluginID,
		RegistrationState: snapshot.RegistrationState,
		DesiredState:      snapshot.DesiredState,
		RuntimeState:      snapshot.RuntimeState,
		DisplayState:      snapshot.DisplayState,
		Commands:          pluginStateEventCommands(snapshot.Commands),
		CommandConflicts:  pluginStateEventCommandConflicts(snapshot, snapshots),
	})
}

func pluginSnapshotsForConflicts(catalog *plugins.Catalog) []plugins.Snapshot {
	if catalog == nil {
		return nil
	}
	return catalog.List()
}

func pluginStateEventCommands(commands []plugins.Command) []pluginCommandEventItem {
	if len(commands) == 0 {
		return []pluginCommandEventItem{}
	}
	items := make([]pluginCommandEventItem, 0, len(commands))
	for _, command := range commands {
		if command.Name == "" {
			continue
		}
		item := pluginCommandEventItem{
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
		return []pluginCommandEventItem{}
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
	return plugins.CommandSourceManifest
}
