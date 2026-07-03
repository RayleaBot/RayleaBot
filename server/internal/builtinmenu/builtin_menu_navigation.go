package menu

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func builtinRootMenuData(items []map[string]any, cfg config.Config) map[string]any {
	rows := make([]map[string]any, 0, len(items))
	firstTarget := ""
	for _, item := range items {
		help, _ := item["help"].(map[string]any)
		target := firstBuiltinMenuText(stringValueFromMap(item, "name"), stringValueFromMap(item, "id"))
		if firstTarget == "" {
			firstTarget = target
		}
		rows = append(rows, map[string]any{
			"name":        stringValueFromMap(item, "name"),
			"description": firstBuiltinMenuText(stringValueFromMap(item, "description"), stringValueFromMap(help, "summary"), "可用插件菜单"),
		})
	}
	return map[string]any{
		"title":            "插件菜单",
		"subtitle":         "当前可用插件",
		"command_prefixes": builtinMenuPrefixes(cfg),
		"trigger_examples": builtinMenuTriggerExamples(firstTarget, cfg),
		"items":            rows,
	}
}

func builtinMenuTriggerExamples(target string, cfg config.Config) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	if len(prefixes) == 0 || len(commands) == 0 {
		return nil
	}
	examples := []string{strings.TrimSpace(prefixes[0] + commands[0] + " " + target)}
	if len(commands) > 1 {
		prefix := prefixes[0]
		if len(prefixes) > 1 {
			prefix = prefixes[1]
		}
		examples = append(examples, strings.TrimSpace(prefix+target+commands[1]))
	}
	return examples
}

func builtinPluginMenuData(item map[string]any, cfg config.Config) map[string]any {
	title := stringValueFromMap(item, "name")
	subtitle := stringValueFromMap(item, "description")
	commands, _ := item["commands"].([]map[string]any)
	groups := make([]map[string]any, 0, 2)
	helpGroups := []map[string]any{}
	if help, ok := item["help"].(map[string]any); ok {
		commands = builtinCommandsNotCoveredByHelp(commands, builtinHelpCommandNames(help))
		help = applyBuiltinHelpCommandPrefixes(help, cfg)
		if values, ok := help["groups"].([]map[string]any); ok {
			helpGroups = values
		}
	}
	if len(commands) > 0 {
		groups = append(groups, map[string]any{
			"title": "命令",
			"items": commands,
		})
	}
	groups = append(groups, helpGroups...)
	return map[string]any{
		"title":            title,
		"subtitle":         subtitle,
		"plugin_name":      stringValueFromMap(item, "plugin_name"),
		"plugin_version":   stringValueFromMap(item, "plugin_version"),
		"command_prefixes": builtinMenuPrefixes(cfg),
		"groups":           groups,
	}
}

func builtinCommandsNotCoveredByHelp(commands []map[string]any, helpCommandNames map[string]struct{}) []map[string]any {
	if len(commands) == 0 || len(helpCommandNames) == 0 {
		return commands
	}
	items := make([]map[string]any, 0, len(commands))
	for _, commandItem := range commands {
		if builtinCommandCoveredByHelp(commandItem, helpCommandNames) {
			continue
		}
		items = append(items, commandItem)
	}
	return items
}

func builtinCommandCoveredByHelp(commandItem map[string]any, helpCommandNames map[string]struct{}) bool {
	for _, value := range append([]string{
		stringValueFromMap(commandItem, "name"),
		stringValueFromMap(commandItem, "declaration_id"),
	}, stringSliceFromMap(commandItem, "aliases")...) {
		if _, ok := helpCommandNames[normalizeMenuLookup(value)]; ok {
			return true
		}
	}
	return false
}

func builtinHelpCommandNames(help map[string]any) map[string]struct{} {
	names := map[string]struct{}{}
	groups, _ := help["groups"].([]map[string]any)
	for _, group := range groups {
		items, _ := group["items"].([]map[string]any)
		for _, item := range items {
			name := normalizeMenuLookup(stringValueFromMap(item, "command_name"))
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func findBuiltinMenuItem(items []map[string]any, target string) (map[string]any, bool) {
	target = normalizeMenuLookup(target)
	for _, item := range items {
		if target == normalizeMenuLookup(stringValueFromMap(item, "id")) || target == normalizeMenuLookup(stringValueFromMap(item, "name")) {
			return item, true
		}
		commands, _ := item["commands"].([]map[string]any)
		for _, commandItem := range commands {
			if target == normalizeMenuLookup(stringValueFromMap(commandItem, "name")) ||
				target == normalizeMenuLookup(stringValueFromMap(commandItem, "declaration_id")) {
				return item, true
			}
			for _, alias := range stringSliceFromMap(commandItem, "aliases") {
				if target == normalizeMenuLookup(alias) {
					return item, true
				}
			}
		}
	}
	return nil, false
}
