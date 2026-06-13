package plugins

import (
	"sort"
	"strings"
)

func DetectCommandConflicts(snapshots []Snapshot) map[string][]string {
	owners := make(map[string]map[string]struct{})
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		seen := make(map[string]struct{})
		for _, command := range snapshot.Commands {
			addSummaryConflictToken(seen, command.Name)
			for _, alias := range command.Aliases {
				addSummaryConflictToken(seen, alias)
			}
		}
		for token := range seen {
			if owners[token] == nil {
				owners[token] = make(map[string]struct{})
			}
			owners[token][snapshot.PluginID] = struct{}{}
		}
	}

	conflicts := make(map[string][]string)
	for token, pluginIDs := range owners {
		if len(pluginIDs) < 2 {
			continue
		}
		for pluginID := range pluginIDs {
			conflicts[pluginID] = append(conflicts[pluginID], token)
		}
	}
	for pluginID := range conflicts {
		sort.Strings(conflicts[pluginID])
	}
	return conflicts
}

func normalizeConflictViews(conflicts []string) []string {
	if len(conflicts) == 0 {
		return []string{}
	}
	return append([]string(nil), conflicts...)
}

func buildCommandViews(snapshot Snapshot) []CommandView {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || len(snapshot.Commands) == 0 {
		return []CommandView{}
	}
	items := make([]CommandView, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		items = append(items, CommandView{
			Name:          command.Name,
			Aliases:       normalizeStringViews(command.Aliases),
			Description:   strings.TrimSpace(command.Description),
			Usage:         strings.TrimSpace(command.Usage),
			Permission:    strings.TrimSpace(command.Permission),
			CommandSource: strings.TrimSpace(command.CommandSource),
			DeclarationID: strings.TrimSpace(command.DeclarationID),
		})
	}
	return items
}

func buildHelpView(snapshot Snapshot) *HelpView {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.Help == nil {
		return nil
	}

	help := &HelpView{
		Title:   strings.TrimSpace(snapshot.Help.Title),
		Summary: strings.TrimSpace(snapshot.Help.Summary),
	}
	for _, group := range snapshot.Help.Groups {
		title := strings.TrimSpace(group.Title)
		if title == "" {
			continue
		}
		viewGroup := HelpGroupView{Title: title}
		for _, item := range group.Items {
			itemTitle := strings.TrimSpace(item.Title)
			if itemTitle == "" {
				continue
			}
			viewGroup.Items = append(viewGroup.Items, HelpItemView{
				Title:       itemTitle,
				Description: strings.TrimSpace(item.Description),
				Usage:       strings.TrimSpace(item.Usage),
				Command:     strings.TrimSpace(item.Command),
				Permission:  strings.TrimSpace(item.Permission),
			})
		}
		if len(viewGroup.Items) > 0 {
			help.Groups = append(help.Groups, viewGroup)
		}
	}
	if help.Title == "" && help.Summary == "" && len(help.Groups) == 0 {
		return nil
	}
	return help
}

func normalizeStringViews(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func addSummaryConflictToken(tokens map[string]struct{}, raw string) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return
	}
	tokens[token] = struct{}{}
}
