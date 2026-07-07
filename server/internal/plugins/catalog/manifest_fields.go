package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func extractStringField(document map[string]any, key string) (string, bool) {
	value, ok := document[key]
	if !ok {
		return "", false
	}

	stringValue, ok := value.(string)
	if !ok || stringValue == "" {
		return "", false
	}

	return stringValue, true
}

func stringField(document map[string]any, key string) string {
	value, ok := document[key]
	if !ok {
		return ""
	}

	stringValue, ok := value.(string)
	if !ok {
		return ""
	}

	return stringValue
}

func manifestBoolField(document map[string]any, key string) bool {
	value, ok := document[key]
	if !ok {
		return false
	}
	booleanValue, ok := value.(bool)
	if !ok {
		return false
	}
	return booleanValue
}

func manifestConcurrency(document map[string]any) int {
	value, ok := document["concurrency"]
	if !ok {
		return 1
	}
	switch typed := value.(type) {
	case int:
		if typed >= 1 {
			return typed
		}
	case int64:
		if typed >= 1 {
			return int(typed)
		}
	case float64:
		if typed >= 1 {
			return int(typed)
		}
	}
	return 1
}

func manifestRole(document map[string]any, sourceRoot string) string {
	role := stringField(document, "role")
	if role != "" {
		return role
	}

	switch sourceRoot {
	case "plugins/builtin":
		return "builtin"
	case "examples/plugins":
		return "example"
	case "plugins/dev":
		return "dev"
	default:
		return "user"
	}
}

func manifestObjectField(document map[string]any, key string) map[string]any {
	value, ok := document[key].(map[string]any)
	if !ok {
		return nil
	}
	return plugins.CloneSettings(value)
}

func defaultDesiredStateForSourceRoot(sourceRoot string) string {
	if sourceRoot == "plugins/builtin" {
		return plugins.DesiredStateEnabled
	}
	return plugins.DesiredStateDisabled
}

func stringListField(document map[string]any, key string) []string {
	values, ok := document[key].([]any)
	if !ok {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		items = append(items, text)
	}
	return items
}

func manifestDefaultConfig(document map[string]any, packageRoot string) (map[string]any, error) {
	fileConfig, err := manifestDefaultConfigFile(document, packageRoot)
	if err != nil {
		return nil, err
	}

	inlineConfig := manifestObjectField(document, "default_config")
	if len(fileConfig) == 0 {
		return inlineConfig, nil
	}
	if len(inlineConfig) == 0 {
		return fileConfig, nil
	}

	merged := plugins.CloneSettings(fileConfig)
	for key, value := range inlineConfig {
		merged[key] = plugins.CloneSettingValue(value)
	}
	return merged, nil
}

func manifestDefaultConfigFile(document map[string]any, packageRoot string) (map[string]any, error) {
	relativePath := stringField(document, "default_config_file")
	if relativePath == "" {
		return nil, nil
	}
	if filepath.IsAbs(relativePath) {
		return nil, fmt.Errorf("default_config_file must be package-relative")
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}
	if filepath.Ext(cleanRelative) != ".json" {
		return nil, fmt.Errorf("default_config_file must point to a .json file")
	}

	packageRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin package root: %w", err)
	}
	configPath := filepath.Join(packageRoot, cleanRelative)
	if !pathWithinRoot(packageRoot, configPath) {
		return nil, fmt.Errorf("default_config_file must stay inside the plugin package")
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read default_config_file %s: %w", relativePath, err)
	}

	var value any
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, fmt.Errorf("parse default_config_file %s: %w", relativePath, err)
	}

	config, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("default_config_file %s must contain a JSON object", relativePath)
	}
	return plugins.CloneSettings(config), nil
}

func pathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (relativePath != "" && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}

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

func manifestScreenshots(document map[string]any) []plugins.Screenshot {
	values, ok := document["screenshots"].([]any)
	if !ok {
		return nil
	}

	items := make([]plugins.Screenshot, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		path := stringField(item, "path")
		if path == "" {
			continue
		}
		items = append(items, plugins.Screenshot{
			Path: path,
			Alt:  stringField(item, "alt"),
		})
	}
	return items
}

func manifestManagementUI(document map[string]any) *plugins.ManagementUI {
	value, ok := document["management_ui"].(map[string]any)
	if !ok {
		return nil
	}

	pages := manifestManagementUIPages(value)
	if len(pages) == 0 {
		return nil
	}

	return &plugins.ManagementUI{
		Pages: pages,
	}
}

func manifestManagementUIPages(document map[string]any) []plugins.ManagementUIPage {
	values, ok := document["pages"].([]any)
	if !ok {
		return nil
	}

	items := make([]plugins.ManagementUIPage, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, plugins.ManagementUIPage{
			ID:    stringField(item, "id"),
			Label: stringField(item, "label"),
			Entry: stringField(item, "entry"),
		})
	}
	return items
}

func validateManagementUIPages(managementUI *plugins.ManagementUI) error {
	if managementUI == nil || len(managementUI.Pages) == 0 {
		return nil
	}

	assetRoot := pathDirectory(managementUI.Pages[0].Entry)
	seen := map[string]struct{}{}
	for _, page := range managementUI.Pages {
		if _, exists := seen[page.ID]; exists {
			return fmt.Errorf("management_ui.pages contains duplicate id %q", page.ID)
		}
		seen[page.ID] = struct{}{}
		if pathDirectory(page.Entry) != assetRoot {
			return fmt.Errorf("management_ui.pages entry %q must stay inside %q", page.Entry, assetRoot)
		}
	}
	return nil
}

func pathDirectory(value string) string {
	cleaned := strings.TrimSpace(filepath.ToSlash(value))
	index := strings.LastIndex(cleaned, "/")
	if index < 0 {
		return ""
	}
	return cleaned[:index]
}

func manifestRenderTemplates(document map[string]any) []plugins.RenderTemplate {
	values, ok := document["render_templates"].([]any)
	if !ok {
		return nil
	}

	items := make([]plugins.RenderTemplate, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		path := stringField(item, "path")
		if path == "" {
			continue
		}
		items = append(items, plugins.RenderTemplate{Path: path})
	}
	return items
}

func manifestHelp(document map[string]any) *plugins.Help {
	value, ok := document["help"].(map[string]any)
	if !ok {
		return nil
	}

	help := &plugins.Help{
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
		group := plugins.HelpGroup{
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
			group.Items = append(group.Items, plugins.HelpItem{
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

func manifestCommands(document map[string]any) []plugins.Command {
	values, ok := document["commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]plugins.Command, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := stringField(item, "name")
		if name == "" {
			continue
		}
		command := plugins.Command{
			Name:          name,
			Aliases:       stringListField(item, "aliases"),
			Description:   stringField(item, "description"),
			Usage:         stringField(item, "usage"),
			Permission:    stringField(item, "permission"),
			CommandSource: plugins.CommandSourceManifest,
		}
		commands = append(commands, command)
	}
	return commands
}

func manifestDynamicCommands(document map[string]any) []plugins.DynamicCommandDecl {
	values, ok := document["dynamic_commands"].([]any)
	if !ok {
		return nil
	}

	commands := make([]plugins.DynamicCommandDecl, 0, len(values))
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
		commands = append(commands, plugins.DynamicCommandDecl{
			ID:          id,
			SettingsKey: settingsKey,
			Description: stringField(item, "description"),
			UsageArgs:   stringField(item, "usage_args"),
			Permission:  stringField(item, "permission"),
		})
	}
	return commands
}

func manifestCommandPatterns(document map[string]any) []plugins.CommandPatternDecl {
	values, ok := document["command_patterns"].([]any)
	if !ok {
		return nil
	}

	commands := make([]plugins.CommandPatternDecl, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id := stringField(item, "id")
		name := stringField(item, "name")
		pattern := stringField(item, "pattern")
		if id == "" || name == "" || pattern == "" {
			continue
		}
		commands = append(commands, plugins.CommandPatternDecl{
			ID:          id,
			Name:        name,
			Pattern:     pattern,
			Description: stringField(item, "description"),
			Usage:       stringField(item, "usage"),
			Permission:  stringField(item, "permission"),
		})
	}
	return commands
}
