package pluginmanifest

func manifestHelp(document map[string]any) *Help {
	value, ok := document["help"].(map[string]any)
	if !ok {
		return nil
	}

	help := &Help{
		Title:   stringField(value, "title"),
		Summary: stringField(value, "summary"),
	}
	groups, ok := value["groups"].([]any)
	if !ok {
		return help
	}

	for _, rawGroup := range groups {
		groupMap, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		group := HelpGroup{
			Title: stringField(groupMap, "title"),
		}
		rawItems, ok := groupMap["items"].([]any)
		if !ok {
			continue
		}
		for _, rawItem := range rawItems {
			itemMap, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			title := stringField(itemMap, "title")
			if title == "" {
				continue
			}
			group.Items = append(group.Items, HelpItem{
				Title:       title,
				Description: stringField(itemMap, "description"),
				Usage:       stringField(itemMap, "usage"),
				Command:     stringField(itemMap, "command"),
				Permission:  stringField(itemMap, "permission"),
			})
		}
		if group.Title != "" && len(group.Items) > 0 {
			help.Groups = append(help.Groups, group)
		}
	}
	return help
}

func manifestCommands(document map[string]any) []Command {
	values, ok := document["commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]Command, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := stringField(item, "name")
		if name == "" {
			continue
		}
		command := Command{
			Name:          name,
			Aliases:       stringListField(item, "aliases"),
			Description:   stringField(item, "description"),
			Usage:         stringField(item, "usage"),
			Permission:    stringField(item, "permission"),
			CommandSource: CommandSourceManifest,
		}
		commands = append(commands, command)
	}
	return commands
}

func manifestDynamicCommands(document map[string]any) []DynamicCommandDecl {
	values, ok := document["dynamic_commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]DynamicCommandDecl, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(item, "id")
		settingsKey := stringField(item, "settings_key")
		if id == "" || settingsKey == "" {
			continue
		}
		commands = append(commands, DynamicCommandDecl{
			ID:          id,
			SettingsKey: settingsKey,
			Description: stringField(item, "description"),
			UsageArgs:   stringField(item, "usage_args"),
			Permission:  stringField(item, "permission"),
		})
	}
	return commands
}
