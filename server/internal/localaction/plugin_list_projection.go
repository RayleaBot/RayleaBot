package localaction

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func buildPluginListCommands(commands []plugins.CommandView) []map[string]any {
	if len(commands) == 0 {
		return []map[string]any{}
	}

	items := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		item := map[string]any{
			"name":           command.Name,
			"command_source": commandSourceOrDefault(command.CommandSource),
		}
		if len(command.Aliases) > 0 {
			item["aliases"] = append([]string(nil), command.Aliases...)
		}
		if command.Description != "" {
			item["description"] = command.Description
		}
		if command.Usage != "" {
			item["usage"] = command.Usage
		}
		if command.Permission != "" {
			item["permission"] = command.Permission
		}
		if command.DeclarationID != "" {
			item["declaration_id"] = command.DeclarationID
		}
		items = append(items, item)
	}
	return items
}

func buildPluginListHelp(help *plugins.HelpView) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	groups := make([]map[string]any, 0, len(help.Groups))
	for _, group := range help.Groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			entry := map[string]any{
				"title": item.Title,
			}
			if item.Description != "" {
				entry["description"] = item.Description
			}
			if item.Usage != "" {
				entry["usage"] = item.Usage
			}
			if item.Command != "" {
				entry["command"] = item.Command
			}
			if item.Permission != "" {
				entry["permission"] = item.Permission
			}
			items = append(items, entry)
		}
		if len(items) == 0 {
			continue
		}
		groups = append(groups, map[string]any{
			"title": group.Title,
			"items": items,
		})
	}
	if len(groups) > 0 {
		result["groups"] = groups
	}
	return result
}

func commandSourceOrDefault(source string) string {
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	return plugins.CommandSourceManifest
}
