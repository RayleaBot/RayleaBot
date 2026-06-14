package pluginmanifest

import (
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"path/filepath"
	"strings"
)

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
