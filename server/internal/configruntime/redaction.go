package configruntime

import (
	"slices"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

const redactedConfigValue = "********"

var secretConfigPaths = [][]string{
	{"onebot", "forward_ws", "access_token"},
	{"onebot", "http_api", "access_token"},
	{"onebot", "reverse_ws", "access_token"},
	{"onebot", "webhook", "access_token"},
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	redactedFields := make([]string, 0, len(secretConfigPaths))
	for _, path := range secretConfigPaths {
		value, ok := lookupConfigPath(cloned, path)
		if !ok || strings.TrimSpace(stringValue(value)) == "" {
			continue
		}
		setConfigPath(cloned, path, redactedConfigValue)
		redactedFields = append(redactedFields, strings.Join(path, "."))
	}
	slices.Sort(redactedFields)
	return cloned, redactedFields
}

func restoreRedactedConfigSecrets(request, current map[string]any) map[string]any {
	cloned := internalconfig.CloneDocument(request)
	if cloned == nil {
		return nil
	}

	for _, path := range secretConfigPaths {
		currentValue, _ := lookupConfigPath(current, path)
		requestValue, exists := lookupConfigPath(cloned, path)
		if exists && strings.TrimSpace(stringValue(requestValue)) != redactedConfigValue {
			continue
		}
		setConfigPath(cloned, path, stringValue(currentValue))
	}
	return cloned
}

func configSecretValues(cfg internalconfig.Config) []string {
	return []string{
		cfg.OneBot.ForwardWS.AccessToken,
		cfg.OneBot.HTTPAPI.AccessToken,
		cfg.OneBot.ReverseWS.AccessToken,
		cfg.OneBot.Webhook.AccessToken,
	}
}

func lookupConfigPath(document map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return document, true
	}

	current := any(document)
	for _, segment := range path {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := currentMap[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func setConfigPath(document map[string]any, path []string, value any) {
	if document == nil || len(path) == 0 {
		return
	}

	current := document
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
