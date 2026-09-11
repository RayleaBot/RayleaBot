package chatpolicy

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
)

type commandPolicyContext struct {
	CommandName      string
	PermissionInfo   *permission.CommandInfo
	MatchedPluginIDs []string
	PrimaryPluginID  string
}

func (s *Service) CommandInfoForEvent(event chatevent.NormalizedEvent) *permission.CommandInfo {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	commandContext := s.commandPolicyContextForEvent(event)
	if commandContext == nil {
		return nil
	}
	return commandContext.PermissionInfo
}

func (s *Service) commandPolicyContextForEvent(event chatevent.NormalizedEvent) *commandPolicyContext {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	requiredLevel := "everyone"
	context := &commandPolicyContext{
		CommandName:    commandName,
		PermissionInfo: &permission.CommandInfo{Permission: requiredLevel},
	}
	defaultLevel := "everyone"
	if s != nil {
		if engine := s.currentEngine(); engine != nil {
			defaultLevel = normalizePermissionLevel(engine.snapshot.DefaultLevel)
		}
	}
	if s != nil && s.plugins != nil {
		for _, snapshot := range s.plugins.List() {
			if !snapshot.CommandsEnabled() {
				continue
			}
			for _, command := range snapshot.Commands {
				if !command.Matches(commandName) {
					continue
				}
				context.MatchedPluginIDs = append(context.MatchedPluginIDs, snapshot.PluginID)
				level := effectiveCommandPermissionLevel(command.Permission, defaultLevel)
				if commandPermissionRank(level) > commandPermissionRank(requiredLevel) {
					requiredLevel = level
				}
				break
			}
		}
	}
	if s != nil && s.menu != nil && s.menu.Match(event).Matched {
		context.MatchedPluginIDs = nil
		requiredLevel = "everyone"
	}

	context.PermissionInfo.Permission = requiredLevel
	if len(context.MatchedPluginIDs) == 1 {
		context.PrimaryPluginID = context.MatchedPluginIDs[0]
	}
	return context
}

func commandNameFromEvent(event chatevent.NormalizedEvent) string {
	if event.PayloadFields == nil {
		return ""
	}
	value, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func effectiveCommandPermissionLevel(permissionLevel string, defaultLevel string) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return defaultLevel
	default:
		return "everyone"
	}
}

func commandPermissionRank(level string) int {
	switch level {
	case "super_admin":
		return 3
	case "group_admin":
		return 2
	case "everyone":
		return 1
	default:
		return 1
	}
}
