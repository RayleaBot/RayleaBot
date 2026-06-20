package manifest

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func manifestDependencyList(document map[string]any, key string) []string {
	dependencies, ok := document["dependencies"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(dependencies, key)
}

func manifestCapabilityParameterList(document map[string]any, key string) []string {
	parameters, ok := document["capability_parameters"].(map[string]any)
	if !ok {
		return nil
	}
	return stringListField(parameters, key)
}

func manifestWebhookParameters(document map[string]any) []plugins.WebhookScope {
	parameters, ok := document["capability_parameters"].(map[string]any)
	if !ok {
		return nil
	}
	values, ok := parameters["webhooks"].([]any)
	if !ok {
		return nil
	}

	items := make([]plugins.WebhookScope, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		scope := plugins.WebhookScope{
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
