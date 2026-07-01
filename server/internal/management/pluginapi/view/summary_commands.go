package view

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func buildPluginCommands(snapshot plugins.Snapshot) []CommandResponse {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || len(snapshot.Commands) == 0 {
		return []CommandResponse{}
	}

	items := make([]CommandResponse, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		items = append(items, CommandResponse{
			Name:          command.Name,
			Aliases:       NormalizeStringList(command.Aliases),
			Description:   strings.TrimSpace(command.Description),
			Usage:         strings.TrimSpace(command.Usage),
			Permission:    strings.TrimSpace(command.Permission),
			CommandSource: commandSourceOrDefault(command.CommandSource),
			DeclarationID: strings.TrimSpace(command.DeclarationID),
		})
	}

	return items
}

func buildPluginHelp(snapshot plugins.Snapshot) HelpResponse {
	helpView := plugins.BuildHelpView(snapshot)
	if helpView == nil {
		return HelpResponse{Groups: []HelpGroupResponse{}}
	}
	result := HelpResponse{
		Title:   helpView.Title,
		Summary: helpView.Summary,
		Groups:  []HelpGroupResponse{},
	}
	for _, group := range helpView.Groups {
		itemGroup := HelpGroupResponse{
			Title: group.Title,
			Items: make([]HelpItemResponse, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			itemGroup.Items = append(itemGroup.Items, HelpItemResponse{
				Title:       item.Title,
				Description: item.Description,
				Usage:       item.Usage,
				Command:     item.Command,
				Permission:  item.Permission,
			})
		}
		if len(itemGroup.Items) > 0 {
			result.Groups = append(result.Groups, itemGroup)
		}
	}
	return result
}

func commandSourceOrDefault(source string) string {
	source = strings.TrimSpace(source)
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	if source == plugins.CommandSourcePattern {
		return plugins.CommandSourcePattern
	}
	return plugins.CommandSourceManifest
}

func NormalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}
