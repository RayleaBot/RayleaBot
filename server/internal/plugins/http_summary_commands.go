package plugins

import "strings"

func buildPluginCommands(snapshot Snapshot) []pluginCommandResponse {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || len(snapshot.Commands) == 0 {
		return []pluginCommandResponse{}
	}

	items := make([]pluginCommandResponse, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		items = append(items, pluginCommandResponse{
			Name:          command.Name,
			Aliases:       normalizeStringList(command.Aliases),
			Description:   strings.TrimSpace(command.Description),
			Usage:         strings.TrimSpace(command.Usage),
			Permission:    strings.TrimSpace(command.Permission),
			CommandSource: commandSourceOrDefault(command.CommandSource),
			DeclarationID: strings.TrimSpace(command.DeclarationID),
		})
	}

	return items
}

func buildPluginHelp(snapshot Snapshot) pluginHelpResponse {
	view := buildHelpView(snapshot)
	if view == nil {
		return pluginHelpResponse{Groups: []pluginHelpGroupResponse{}}
	}
	result := pluginHelpResponse{
		Title:   view.Title,
		Summary: view.Summary,
		Groups:  []pluginHelpGroupResponse{},
	}
	for _, group := range view.Groups {
		itemGroup := pluginHelpGroupResponse{
			Title: group.Title,
			Items: make([]pluginHelpItemResponse, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			itemGroup.Items = append(itemGroup.Items, pluginHelpItemResponse{
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
	if source == CommandSourceDynamic {
		return CommandSourceDynamic
	}
	return CommandSourceManifest
}

func normalizeStringList(values []string) []string {
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
