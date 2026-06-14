package manifest

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"strings"
	"unicode"
)

func ProjectCommands(snapshot plugins.Snapshot, settings map[string]any) []plugins.Command {
	items := make([]plugins.Command, 0, len(snapshot.ManifestCommands)+len(snapshot.DynamicCommands))
	for _, command := range snapshot.ManifestCommands {
		normalized := command
		normalized.CommandSource = CommandSourceManifest
		normalized.DeclarationID = ""
		normalized.Name = strings.TrimSpace(normalized.Name)
		normalized.Aliases = normalizeStaticCommandTokens(normalized.Aliases)
		if normalized.Name == "" {
			continue
		}
		items = append(items, normalized)
	}

	for _, declaration := range snapshot.DynamicCommands {
		tokens, hasSetting := commandTokensFromSetting(settings, declaration.SettingsKey)
		if !hasSetting {
			tokens, _ = commandTokensFromSetting(snapshot.DefaultConfig, declaration.SettingsKey)
		}
		if len(tokens) == 0 {
			continue
		}

		name := tokens[0]
		usage := name
		if usageArgs := strings.TrimSpace(declaration.UsageArgs); usageArgs != "" {
			usage = name + " " + usageArgs
		}
		items = append(items, plugins.Command{
			Name:          name,
			Aliases:       append([]string(nil), tokens[1:]...),
			Description:   strings.TrimSpace(declaration.Description),
			Usage:         usage,
			Permission:    strings.TrimSpace(declaration.Permission),
			CommandSource: CommandSourceDynamic,
			DeclarationID: strings.TrimSpace(declaration.ID),
		})
	}
	return items
}

func normalizeStaticCommandTokens(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := strings.TrimSpace(value)
		if !validStaticCommandToken(token) {
			continue
		}
		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, token)
	}
	return items
}

func commandTokensFromSetting(settings map[string]any, key string) ([]string, bool) {
	key = strings.TrimSpace(key)
	if len(settings) == 0 || key == "" {
		return nil, false
	}
	value, exists := settings[key]
	if !exists {
		return nil, false
	}

	switch typed := value.(type) {
	case string:
		return normalizeDynamicCommandTokens([]string{typed}), true
	case []string:
		return normalizeDynamicCommandTokens(typed), true
	case []any:
		values := make([]string, 0, len(typed))
		for _, value := range typed {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
		return normalizeDynamicCommandTokens(values), true
	default:
		return nil, true
	}
}

func normalizeDynamicCommandTokens(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := strings.TrimSpace(value)
		if !validDynamicCommandToken(token) {
			continue
		}
		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, token)
	}
	return items
}

func validDynamicCommandToken(token string) bool {
	return validStaticCommandToken(token)
}

func validStaticCommandToken(token string) bool {
	if token == "" {
		return false
	}
	return !strings.ContainsFunc(token, unicode.IsSpace)
}
