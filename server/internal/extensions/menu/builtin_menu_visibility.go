package menu

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func visibleBuiltinCommands(commands []plugins.CommandView, cfg config.Config, event runtimeprotocol.Event) []plugins.CommandView {
	callerRank := builtinMenuCallerPermissionRank(cfg, event)
	items := make([]plugins.CommandView, 0, len(commands))
	for _, item := range commands {
		level := builtinMenuEffectiveCommandPermission(item.Permission, cfg)
		if callerRank >= builtinMenuPermissionRank(level) {
			items = append(items, item)
		}
	}
	return items
}

func visibleBuiltinHelp(help *plugins.HelpView, allCommands []plugins.CommandView, visibleCommands []plugins.CommandView, cfg config.Config, event runtimeprotocol.Event) *plugins.HelpView {
	if help == nil {
		return nil
	}
	visibleTokens := builtinMenuCommandTokenSet(visibleCommands)
	allTokens := builtinMenuCommandTokenSet(allCommands)
	commandPermissions := builtinMenuCommandPermissionSet(allCommands, cfg)
	callerRank := builtinMenuCallerPermissionRank(cfg, event)
	filtered := &plugins.HelpView{
		Title:   help.Title,
		Summary: help.Summary,
	}
	for _, group := range help.Groups {
		filteredGroup := plugins.HelpGroupView{Title: group.Title}
		for _, item := range group.Items {
			commandToken := strings.ToLower(strings.TrimSpace(item.Command))
			if commandToken != "" {
				if _, commandExists := allTokens[commandToken]; !commandExists {
					continue
				}
				if _, commandVisible := visibleTokens[commandToken]; !commandVisible {
					continue
				}
			}
			level := builtinMenuEffectiveHelpItemPermission(item, commandPermissions)
			if callerRank >= builtinMenuPermissionRank(level) {
				filteredGroup.Items = append(filteredGroup.Items, item)
			}
		}
		if len(filteredGroup.Items) > 0 {
			filtered.Groups = append(filtered.Groups, filteredGroup)
		}
	}
	if len(filtered.Groups) == 0 {
		return nil
	}
	return filtered
}
