package plugins

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.DisplayState = projectDisplayState(snapshot)
	cloned.DefaultConfig = cloneMap(snapshot.DefaultConfig)
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	cloned.Platforms = append([]string(nil), snapshot.Platforms...)
	cloned.Keywords = append([]string(nil), snapshot.Keywords...)
	cloned.SystemDependencies = append([]string(nil), snapshot.SystemDependencies...)
	cloned.RequiredPermissions = append([]string(nil), snapshot.RequiredPermissions...)
	cloned.OptionalPermissions = append([]string(nil), snapshot.OptionalPermissions...)
	cloned.DeclaredCapabilities = append([]string(nil), snapshot.DeclaredCapabilities...)
	cloned.PythonDependencies = append([]string(nil), snapshot.PythonDependencies...)
	cloned.NodeDependencies = append([]string(nil), snapshot.NodeDependencies...)
	cloned.ScopeHTTPHosts = append([]string(nil), snapshot.ScopeHTTPHosts...)
	cloned.ScopeStorageRoots = append([]string(nil), snapshot.ScopeStorageRoots...)
	if len(snapshot.ScopeWebhooks) > 0 {
		cloned.ScopeWebhooks = make([]WebhookScope, 0, len(snapshot.ScopeWebhooks))
		for _, scope := range snapshot.ScopeWebhooks {
			copied := scope
			copied.SourceIPs = append([]string(nil), scope.SourceIPs...)
			cloned.ScopeWebhooks = append(cloned.ScopeWebhooks, copied)
		}
	}
	if len(snapshot.Screenshots) > 0 {
		cloned.Screenshots = make([]Screenshot, 0, len(snapshot.Screenshots))
		for _, screenshot := range snapshot.Screenshots {
			cloned.Screenshots = append(cloned.Screenshots, screenshot)
		}
	}
	if snapshot.ManagementUI != nil {
		copied := *snapshot.ManagementUI
		copied.Pages = append([]ManagementUIPage(nil), snapshot.ManagementUI.Pages...)
		cloned.ManagementUI = &copied
	}
	if len(snapshot.RenderTemplates) > 0 {
		cloned.RenderTemplates = append([]RenderTemplate(nil), snapshot.RenderTemplates...)
	}
	if snapshot.Help != nil {
		cloned.Help = cloneHelp(snapshot.Help)
	}
	if snapshot.DeadLetter != nil {
		copied := *snapshot.DeadLetter
		cloned.DeadLetter = &copied
	}
	if len(snapshot.Commands) > 0 {
		cloned.Commands = cloneCommands(snapshot.Commands)
	}
	if len(snapshot.ManifestCommands) > 0 {
		cloned.ManifestCommands = cloneCommands(snapshot.ManifestCommands)
	}
	if len(snapshot.DynamicCommands) > 0 {
		cloned.DynamicCommands = append([]DynamicCommandDecl(nil), snapshot.DynamicCommands...)
	}
	return cloned
}

func CloneSnapshot(snapshot Snapshot) Snapshot {
	return cloneSnapshot(snapshot)
}

func cloneHelp(help *Help) *Help {
	if help == nil {
		return nil
	}
	cloned := *help
	if len(help.Groups) > 0 {
		cloned.Groups = make([]HelpGroup, 0, len(help.Groups))
		for _, group := range help.Groups {
			copied := group
			if len(group.Items) > 0 {
				copied.Items = append([]HelpItem(nil), group.Items...)
			}
			cloned.Groups = append(cloned.Groups, copied)
		}
	}
	return &cloned
}

func cloneCommands(commands []Command) []Command {
	if len(commands) == 0 {
		return nil
	}
	cloned := make([]Command, 0, len(commands))
	for _, cmd := range commands {
		copied := cmd
		copied.Aliases = append([]string(nil), cmd.Aliases...)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneSlice(values []any) []any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]any, len(values))
	for i, value := range values {
		cloned[i] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}
