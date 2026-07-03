package menu

import (
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderidentity"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func (s *Service) buildBuiltinMenuData(event adapterintake.NormalizedEvent, target string) builtinMenuRenderData {
	items := s.visibleBuiltinMenuItems(event)
	runtimeEvent := runtimeEventFromAdapter(event)
	cfg := s.config()
	if target != "" {
		if item, ok := findBuiltinMenuItem(items, target); ok {
			data := s.withBuiltinMenuIdentity(builtinPluginMenuData(item, cfg), runtimeEvent)
			return builtinMenuRenderData{
				Data: data,
				Plugin: &renderservice.PluginContext{
					Name:    stringValueFromMap(item, "plugin_name"),
					Version: stringValueFromMap(item, "plugin_version"),
				},
			}
		}
		return builtinMenuRenderData{}
	}
	return builtinMenuRenderData{Data: s.withBuiltinMenuIdentity(builtinRootMenuData(items, cfg), runtimeEvent)}
}

func (s *Service) visibleBuiltinMenuItems(event adapterintake.NormalizedEvent) []map[string]any {
	if s == nil || s.plugins == nil {
		return []map[string]any{}
	}
	runtimeEvent := runtimeEventFromAdapter(event)
	cfg := s.config()
	snapshots := s.plugins.List()
	conflicts := plugins.DetectCommandConflicts(snapshots)
	items := make([]map[string]any, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
			continue
		}
		view := plugins.BuildSummaryView(snapshot, conflicts[snapshot.PluginID])
		commands := visibleBuiltinCommands(view.Commands, cfg, runtimeEvent)
		help := visibleBuiltinHelp(view.Help, view.Commands, commands, cfg, runtimeEvent)
		if len(commands) == 0 && help == nil {
			continue
		}
		item := map[string]any{
			"id":             view.ID,
			"name":           view.Name,
			"plugin_name":    view.Name,
			"plugin_version": view.Version,
			"description":    view.Description,
			"commands":       buildBuiltinCommands(commands, cfg),
		}
		if help != nil {
			item["help"] = buildBuiltinHelp(help, view.Commands, cfg)
		}
		items = append(items, item)
	}
	return items
}

func runtimeEventFromAdapter(event adapterintake.NormalizedEvent) runtimeprotocol.Event {
	result := runtimeprotocol.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtimeprotocol.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &runtimeprotocol.EventTarget{
			Type: event.ConversationType,
			ID:   event.ConversationID,
			Name: event.TargetName,
		},
		MessageID:     event.MessageID,
		PayloadFields: event.PayloadFields,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		result.Message = &runtimeprotocol.EventMessage{PlainText: event.PlainText}
		for _, segment := range event.Segments {
			result.Message.Segments = append(result.Message.Segments, runtimeprotocol.EventSegment{
				Type: segment.Type,
				Data: segment.Data,
			})
		}
	}
	return result
}

func (s *Service) withBuiltinMenuIdentity(data map[string]any, event runtimeprotocol.Event) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	cfg := s.config()
	identity := renderidentity.Data(cfg, event)
	data["user"] = identity.User
	data["permission"] = identity.Permission
	if identity.Group != nil {
		data["group"] = identity.Group
	} else {
		delete(data, "group")
	}
	return data
}
