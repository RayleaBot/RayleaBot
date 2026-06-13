package plugins

func manifestPermissionList(document map[string]any, key string) []string {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(permissions, key)
}

func manifestDependencyList(document map[string]any, key string) []string {
	dependencies, ok := document["dependencies"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(dependencies, key)
}

func manifestScopeList(document map[string]any, key string) []string {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	scopes, ok := permissions["scopes"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(scopes, key)
}

func manifestWebhookScopes(document map[string]any) []WebhookScope {
	permissions, ok := document["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	scopes, ok := permissions["scopes"].(map[string]any)
	if !ok {
		return nil
	}
	values, ok := scopes["webhooks"].([]any)
	if !ok {
		return nil
	}

	items := make([]WebhookScope, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		scope := WebhookScope{
			Route:           stringField(item, "route"),
			AuthStrategy:    stringField(item, "auth_strategy"),
			Header:          stringField(item, "header"),
			SecretRef:       stringField(item, "secret_ref"),
			SignaturePrefix: stringField(item, "signature_prefix"),
			SourceIPs:       stringListField(item, "source_ips"),
		}
		if scope.Route == "" || scope.AuthStrategy == "" || scope.Header == "" || scope.SecretRef == "" {
			continue
		}
		items = append(items, scope)
	}
	return items
}
