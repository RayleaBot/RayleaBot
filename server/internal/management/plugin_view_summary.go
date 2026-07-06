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
	role := effectivePluginRole(snapshot)
	state, diagnosis := plugins.ProjectState(snapshot)
	return SummaryResponse{
		ID:               snapshot.PluginID,
		Name:             pluginDisplayName(snapshot),
		Version:          strings.TrimSpace(snapshot.Version),
		Description:      strings.TrimSpace(snapshot.Description),
		Author:           strings.TrimSpace(snapshot.Author),
		Role:             role,
		State:            state,
		StateDiagnosis:   diagnosis,
		Source:           buildPluginSource(snapshot),
		Trust:            buildPluginTrust(role, snapshot),
		Commands:         buildPluginCommands(snapshot),
		Help:             buildPluginHelp(snapshot),
		CommandConflicts: normalizeConflictList(conflicts),
	}
}

func normalizeConflictList(conflicts []string) []string {
	if len(conflicts) == 0 {
		return []string{}
	}
	return append([]string(nil), conflicts...)
}

func pluginDisplayName(snapshot plugins.Snapshot) string {
	if strings.TrimSpace(snapshot.Name) != "" {
		return snapshot.Name
	}
	return snapshot.PluginID
}

func effectivePluginRole(snapshot plugins.Snapshot) string {
	if strings.TrimSpace(snapshot.Role) != "" {
		return snapshot.Role
	}
	switch snapshot.SourceRoot {
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

func buildPluginSource(snapshot plugins.Snapshot) SourceResponse {
	root := snapshot.SourceRoot
	if root == "" && len(snapshot.SourceRoots) > 0 {
		root = snapshot.SourceRoots[0]
	}
	return SourceResponse{
		Root:              root,
		PackageSourceType: snapshot.PackageSourceType,
		PackageSourceRef:  snapshot.PackageSourceRef,
		Verified:          isVerifiedPluginSource(snapshot),
	}
}

func isVerifiedPluginSource(snapshot plugins.Snapshot) bool {
	switch snapshot.SourceRoot {
	case "plugins/builtin", "examples/plugins", "plugins/dev":
		return true
	default:
		return false
	}
}

func buildPluginTrust(role string, snapshot plugins.Snapshot) TrustResponse {
	switch role {
	case "builtin":
		return TrustResponse{Level: "official", Label: "官方"}
	case "dev":
		return TrustResponse{Level: "development", Label: "开发中"}
	case "example":
		return TrustResponse{Level: "third_party", Label: "示例"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "remote_url" {
			return TrustResponse{Level: "unverified", Label: "未验证来源"}
		}
		return TrustResponse{Level: "third_party", Label: "第三方"}
	}
}

func buildPluginCommands(snapshot plugins.Snapshot) []CommandResponse {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || len(snapshot.Commands) == 0 {
		return []CommandResponse{}
	}

	items := make([]CommandResponse, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		items = append(items, CommandResponse{
			Name:          command.Name,
			Aliases:       NormalizeStringList(command.Aliases),
			Description:   strings.TrimSpace(command.Description),
			Usage:         strings.TrimSpace(command.Usage),
			Permission:    strings.TrimSpace(command.Permission),
			CommandSource: commandSourceOrDefault(command.CommandSource),
			DeclarationID: strings.TrimSpace(command.DeclarationID),
		})
	}

	return items
}

func buildPluginHelp(snapshot plugins.Snapshot) HelpResponse {
	helpView := plugins.BuildHelpView(snapshot)
	if helpView == nil {
		return HelpResponse{Groups: []HelpGroupResponse{}}
	}
	result := HelpResponse{
		Title:   helpView.Title,
		Summary: helpView.Summary,
		Groups:  []HelpGroupResponse{},
	}
	for _, group := range helpView.Groups {
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

func commandSourceOrDefault(source string) string {
	source = strings.TrimSpace(source)
	if source == plugins.CommandSourceDynamic {
		return plugins.CommandSourceDynamic
	}
	if source == plugins.CommandSourcePattern {
		return plugins.CommandSourcePattern
	}
	return plugins.CommandSourceManifest
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
