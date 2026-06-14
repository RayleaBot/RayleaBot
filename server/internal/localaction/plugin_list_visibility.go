package localaction

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func (s *Service) visiblePluginListCommandsForCaller(commands []plugins.CommandView, event runtimeprotocol.Event) []plugins.CommandView {
	if len(commands) == 0 {
		return []plugins.CommandView{}
	}

	cfg := s.config()
	callerRank := pluginListCallerPermissionRank(cfg, event)
	visible := make([]plugins.CommandView, 0, len(commands))
	for _, command := range commands {
		level := pluginListEffectiveCommandPermission(command.Permission, cfg)
		if callerRank >= pluginListPermissionRank(level) {
			visible = append(visible, command)
		}
	}
	return visible
}

func (s *Service) visiblePluginListHelpForCaller(help *plugins.HelpView, allCommands []plugins.CommandView, visibleCommands []plugins.CommandView, event runtimeprotocol.Event) *plugins.HelpView {
	if help == nil {
		return nil
	}

	visibleTokens := pluginListCommandTokenSet(visibleCommands)
	allTokens := pluginListCommandTokenSet(allCommands)
	cfg := s.config()
	callerRank := pluginListCallerPermissionRank(cfg, event)
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
				filteredGroup.Items = append(filteredGroup.Items, item)
				continue
			}

			level := pluginListEffectiveHelpPermission(item.Permission)
			if callerRank >= pluginListPermissionRank(level) {
				filteredGroup.Items = append(filteredGroup.Items, item)
			}
		}
		if len(filteredGroup.Items) > 0 {
			filtered.Groups = append(filtered.Groups, filteredGroup)
		}
	}
	if filtered.Title == "" && filtered.Summary == "" && len(filtered.Groups) == 0 {
		return nil
	}
	if len(filtered.Groups) == 0 {
		return nil
	}
	return filtered
}

func pluginListCommandTokenSet(commands []plugins.CommandView) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, command := range commands {
		addPluginListCommandToken(tokens, command.Name)
		addPluginListCommandToken(tokens, command.DeclarationID)
		for _, alias := range command.Aliases {
			addPluginListCommandToken(tokens, alias)
		}
	}
	return tokens
}

func addPluginListCommandToken(tokens map[string]struct{}, value string) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return
	}
	tokens[value] = struct{}{}
}
