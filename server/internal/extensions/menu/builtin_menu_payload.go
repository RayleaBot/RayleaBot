package menu

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func buildBuiltinCommands(commands []plugins.CommandView, cfg config.Config) []map[string]any {
	items := make([]map[string]any, 0, len(commands))
	prefixes := builtinMenuPrefixes(cfg)
	for _, command := range commands {
		item := map[string]any{
			"name":             command.Name,
			"command_prefixes": append([]string(nil), prefixes...),
			"description":      firstBuiltinMenuText(command.Description, command.Name),
			"permission":       builtinMenuEffectiveCommandPermission(command.Permission, cfg),
		}
		if len(command.Aliases) > 0 {
			item["aliases"] = append([]string(nil), command.Aliases...)
		}
		if strings.TrimSpace(command.DeclarationID) != "" {
			item["declaration_id"] = strings.TrimSpace(command.DeclarationID)
		}
		item["permission_label"] = builtinMenuPermissionLabel(stringValueFromMap(item, "permission"))
		items = append(items, item)
	}
	return items
}

func buildBuiltinHelp(help *plugins.HelpView, commands []plugins.CommandView, cfg config.Config) map[string]any {
	result := map[string]any{}
	if help.Title != "" {
		result["title"] = help.Title
	}
	if help.Summary != "" {
		result["summary"] = help.Summary
	}
	commandPermissions := builtinMenuCommandPermissionSet(commands, cfg)
	groups := make([]map[string]any, 0, len(help.Groups))
	for _, group := range help.Groups {
		items := make([]map[string]any, 0, len(group.Items))
		for _, item := range group.Items {
			commandName := strings.TrimSpace(item.Command)
			permission := builtinMenuEffectiveHelpItemPermission(item, commandPermissions)
			entry := map[string]any{
				"name":        firstBuiltinMenuText(commandName, item.Title),
				"title":       item.Title,
				"description": firstBuiltinMenuText(item.Description, item.Title, item.Command),
				"usage":       item.Usage,
				"permission":  permission,
			}
			if commandName != "" {
				entry["command_name"] = commandName
				if usageArgs := builtinCommandUsageArgs(commandName, item.Usage); usageArgs != "" {
					entry["usage_args"] = usageArgs
				}
			}
			entry["permission_label"] = builtinMenuPermissionLabel(stringValueFromMap(entry, "permission"))
			items = append(items, entry)
		}
		if len(items) > 0 {
			groups = append(groups, map[string]any{
				"title": group.Title,
				"items": items,
			})
		}
	}
	if len(groups) > 0 {
		result["groups"] = groups
	}
	return result
}

func applyBuiltinHelpCommandPrefixes(help map[string]any, cfg config.Config) map[string]any {
	prefixes := builtinMenuPrefixes(cfg)
	groups, _ := help["groups"].([]map[string]any)
	for _, group := range groups {
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			if strings.TrimSpace(stringValueFromMap(item, "command_name")) == "" {
				continue
			}
			item["command_prefixes"] = append([]string(nil), prefixes...)
			delete(item, "usage")
			delete(item, "command_name")
		}
	}
	return help
}
