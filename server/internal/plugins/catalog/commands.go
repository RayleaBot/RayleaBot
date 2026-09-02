package catalog

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func ProjectCommands(snapshot plugins.Snapshot, settings map[string]any) []plugins.Command {
	items := make([]plugins.Command, 0, len(snapshot.ManifestCommands))
	for _, declaration := range snapshot.ManifestCommands {
		normalized := declaration
		normalized.ID = strings.TrimSpace(declaration.ID)
		normalized.DisplayName = strings.TrimSpace(declaration.DisplayName)
		normalized.Description = strings.TrimSpace(declaration.Description)
		normalized.Usage = strings.TrimSpace(declaration.Usage)
		normalized.Permission = strings.TrimSpace(declaration.Permission)
		switch declaration.TriggerType {
		case "exact":
			tokens := normalizeStaticCommandTokens(declaration.TriggerNames)
			if len(tokens) == 0 {
				continue
			}
			normalized.Name = tokens[0]
			normalized.Aliases = append([]string(nil), tokens[1:]...)
			normalized.MatchPattern = ""
			items = append(items, normalized)
		case "pattern":
			pattern := strings.TrimSpace(declaration.MatchPattern)
			if !validCommandPattern(pattern) {
				continue
			}
			normalized.Name = normalized.DisplayName
			normalized.Aliases = nil
			normalized.MatchPattern = pattern
			items = append(items, normalized)
		case "setting":
			tokens, hasSetting := commandTokensFromSetting(settings, declaration.SettingsKey)
			if !hasSetting {
				tokens, _ = commandTokensFromSetting(snapshot.DefaultConfig, declaration.SettingsKey)
			}
			if len(tokens) == 0 {
				continue
			}
			normalized.Name = tokens[0]
			normalized.Aliases = append([]string(nil), tokens[1:]...)
			normalized.MatchPattern = ""
			items = append(items, normalized)
		}
	}
	return items
}

func validCommandPattern(pattern string) bool {
	if pattern == "" {
		return false
	}
	_, err := regexp.Compile(pattern)
	return err == nil
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
