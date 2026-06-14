package chatpolicy

import (
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type commandPolicyContext struct {
	CommandName      string
	PermissionInfo   *permission.CommandInfo
	MatchedPluginIDs []string
	PrimaryPluginID  string
}

func (s *Service) CommandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
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

func (s *Service) commandPolicyContextForEvent(event adapterintake.NormalizedEvent) *commandPolicyContext {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	requiredLevel := "everyone"
	context := &commandPolicyContext{
		CommandName:    commandName,
		PermissionInfo: &permission.CommandInfo{Permission: requiredLevel},
	}
	currentConfig := config.Config{}
	if s != nil {
		currentConfig = s.config()
	}
	if s != nil && s.plugins != nil {
		for _, snapshot := range s.plugins.List() {
			if !pluginParticipatesInCommandPolicy(snapshot) {
				continue
			}
			for _, command := range snapshot.Commands {
				if !commandMatches(command, commandName) {
					continue
				}
				context.MatchedPluginIDs = append(context.MatchedPluginIDs, snapshot.PluginID)
				level := effectiveCommandPermissionLevel(command.Permission, currentConfig)
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

func commandNameFromEvent(event adapterintake.NormalizedEvent) string {
	if event.PayloadFields == nil {
		return ""
	}
	value, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func pluginParticipatesInCommandPolicy(snapshot plugins.Snapshot) bool {
	return snapshot.Valid &&
		snapshot.RegistrationState == "installed" &&
		snapshot.DesiredState == "enabled"
}

func commandMatches(command plugins.Command, commandName string) bool {
	if strings.TrimSpace(command.Name) == commandName {
		return true
	}
	for _, alias := range command.Aliases {
		if strings.TrimSpace(alias) == commandName {
			return true
		}
	}
	return false
}

func effectiveCommandPermissionLevel(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return commandPermissionDefaultLevel(cfg)
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
