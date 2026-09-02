package management

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type SummaryResponse struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Version          string                  `json:"version,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Author           string                  `json:"author,omitempty"`
	Icon             string                  `json:"icon,omitempty"`
	Role             string                  `json:"role"`
	State            string                  `json:"state"`
	StateDiagnosis   *plugins.StateDiagnosis `json:"state_diagnosis,omitempty"`
	Source           SourceResponse          `json:"source"`
	Trust            TrustResponse           `json:"trust"`
	Commands         []CommandResponse       `json:"commands"`
	CommandGroups    []CommandGroupResponse  `json:"command_groups"`
	Help             HelpResponse            `json:"help"`
	CommandConflicts []string                `json:"command_conflicts"`
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

type SourceResponse struct {
	Root              string `json:"root"`
	PackageSourceType string `json:"package_source_type,omitempty"`
	PackageSourceRef  string `json:"package_source_ref,omitempty"`
	Verified          bool   `json:"verified"`
}

type TrustResponse struct {
	Level string `json:"level"`
	Label string `json:"label"`
}

type HelpResponse struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type ListResponse struct {
	Items []SummaryResponse `json:"items"`
}

func BuildSummary(catalog plugins.CatalogView, snapshot plugins.Snapshot) SummaryResponse {
	if catalog == nil {
		return ToSummary(snapshot, nil)
	}
	conflicts := plugins.DetectCommandConflicts(catalog.List())
	return ToSummary(snapshot, conflicts[snapshot.PluginID])
}

func ToSummary(snapshot plugins.Snapshot, conflicts []string) SummaryResponse {
	view := plugins.BuildSummaryView(snapshot, conflicts)
	return SummaryResponse{
		ID:               view.ID,
		Name:             view.Name,
		Version:          view.Version,
		Description:      view.Description,
		Author:           view.Author,
		Icon:             view.Icon,
		Role:             view.Role,
		State:            view.State,
		StateDiagnosis:   view.StateDiagnosis,
		Source:           SourceResponse(view.Source),
		Trust:            TrustResponse(view.Trust),
		Commands:         toCommandResponses(view.Commands),
		CommandGroups:    toCommandGroupResponses(view.CommandGroups),
		Help:             toHelpResponse(view.Help),
		CommandConflicts: view.CommandConflicts,
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

func toHelpResponse(help *plugins.HelpView) HelpResponse {
	if help == nil {
		return HelpResponse{}
	}
	return HelpResponse{
		Title:   help.Title,
		Summary: help.Summary,
	}
}

func NormalizeStringList(values []string) []string {
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
