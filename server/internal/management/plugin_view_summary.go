package management

import (
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type SummaryResponse struct {
	plugins.Summary
	Commands      []CommandResponse      `json:"commands"`
	CommandGroups []CommandGroupResponse `json:"command_groups"`
	Help          plugins.HelpView       `json:"help"`
}

type CommandResponse struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	EffectiveNames []string               `json:"effective_names"`
	Description    string                 `json:"description"`
	Usage          string                 `json:"usage"`
	Permission     string                 `json:"permission"`
	Trigger        CommandTriggerResponse `json:"trigger"`
}

type CommandTriggerResponse struct {
	Type        string   `json:"type"`
	Names       []string `json:"names,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	SettingsKey string   `json:"settings_key,omitempty"`
}

type CommandGroupResponse struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Commands []string `json:"commands"`
}

type ListResponse struct {
	pagination.Metadata
	Items []SummaryResponse `json:"items"`
}

func buildSummary(catalog plugins.CatalogView, snapshot plugins.Snapshot) SummaryResponse {
	if catalog == nil {
		return toSummary(snapshot, nil)
	}
	conflicts := plugins.DetectCommandConflicts(catalog.List())
	return toSummary(snapshot, conflicts[snapshot.PluginID])
}

func toSummary(snapshot plugins.Snapshot, conflicts []string) SummaryResponse {
	view := plugins.BuildSummaryView(snapshot, conflicts)
	return SummaryResponse{
		Summary:       view.Summary,
		Commands:      toCommandResponses(view.Commands),
		CommandGroups: toCommandGroupResponses(view.CommandGroups),
		Help:          toHelpResponse(view.Help),
	}
}

func toCommandResponses(commands []plugins.CommandView) []CommandResponse {
	items := make([]CommandResponse, 0, len(commands))
	for _, command := range commands {
		items = append(items, CommandResponse{
			ID:             command.ID,
			Name:           command.Name,
			EffectiveNames: plugins.EffectiveCommandNames(command.TriggerType, command.EffectiveName, command.Aliases),
			Description:    command.Description,
			Usage:          command.Usage,
			Permission:     command.Permission,
			Trigger: CommandTriggerResponse{
				Type: command.TriggerType, Names: command.TriggerNames,
				Pattern: command.MatchPattern, SettingsKey: command.SettingsKey,
			},
		})
	}
	return items
}

func toCommandGroupResponses(groups []plugins.CommandGroup) []CommandGroupResponse {
	items := make([]CommandGroupResponse, 0, len(groups))
	for _, group := range groups {
		items = append(items, CommandGroupResponse{
			ID: group.ID, Title: group.Title, Commands: append([]string(nil), group.Commands...),
		})
	}
	return items
}

func toHelpResponse(help *plugins.HelpView) plugins.HelpView {
	if help == nil {
		return plugins.HelpView{}
	}
	return *help
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}
