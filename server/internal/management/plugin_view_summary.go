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
	Role             string                  `json:"role"`
	State            string                  `json:"state"`
	StateDiagnosis   *plugins.StateDiagnosis `json:"state_diagnosis,omitempty"`
	Source           SourceResponse          `json:"source"`
	Trust            TrustResponse           `json:"trust"`
	Commands         []CommandResponse       `json:"commands"`
	Help             HelpResponse            `json:"help"`
	CommandConflicts []string                `json:"command_conflicts"`
}

type CommandResponse struct {
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases,omitempty"`
	Description   string   `json:"description,omitempty"`
	Usage         string   `json:"usage,omitempty"`
	Permission    string   `json:"permission,omitempty"`
	CommandSource string   `json:"command_source"`
	DeclarationID string   `json:"declaration_id,omitempty"`
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
	Title   string              `json:"title,omitempty"`
	Summary string              `json:"summary,omitempty"`
	Groups  []HelpGroupResponse `json:"groups"`
}

type HelpGroupResponse struct {
	Title string             `json:"title"`
	Items []HelpItemResponse `json:"items"`
}

type HelpItemResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Usage       string `json:"usage,omitempty"`
	Command     string `json:"command,omitempty"`
	Permission  string `json:"permission,omitempty"`
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
		Role:             view.Role,
		State:            view.State,
		StateDiagnosis:   view.StateDiagnosis,
		Source:           SourceResponse(view.Source),
		Trust:            TrustResponse(view.Trust),
		Commands:         toCommandResponses(view.Commands),
		Help:             toHelpResponse(view.Help),
		CommandConflicts: view.CommandConflicts,
	}
}

func toCommandResponses(commands []plugins.CommandView) []CommandResponse {
	items := make([]CommandResponse, 0, len(commands))
	for _, command := range commands {
		items = append(items, CommandResponse{
			Name:          command.Name,
			Aliases:       command.Aliases,
			Description:   command.Description,
			Usage:         command.Usage,
			Permission:    command.Permission,
			CommandSource: command.CommandSource,
			DeclarationID: command.DeclarationID,
		})
	}
	return items
}

func toHelpResponse(help *plugins.HelpView) HelpResponse {
	if help == nil {
		return HelpResponse{Groups: []HelpGroupResponse{}}
	}
	result := HelpResponse{
		Title:   help.Title,
		Summary: help.Summary,
		Groups:  []HelpGroupResponse{},
	}
	for _, group := range help.Groups {
		itemGroup := HelpGroupResponse{
			Title: group.Title,
			Items: make([]HelpItemResponse, 0, len(group.Items)),
		}
		for _, item := range group.Items {
			itemGroup.Items = append(itemGroup.Items, HelpItemResponse{
				Title:       item.Title,
				Description: item.Description,
				Usage:       item.Usage,
				Command:     item.Command,
				Permission:  item.Permission,
			})
		}
		if len(itemGroup.Items) > 0 {
			result.Groups = append(result.Groups, itemGroup)
		}
	}
	return result
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
