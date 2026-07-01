package plugins

import (
	"sort"
	"strings"
)

type CommandView struct {
	Name          string
	Aliases       []string
	Description   string
	Usage         string
	Permission    string
	CommandSource string
	DeclarationID string
}

type HelpView struct {
	Title   string
	Summary string
	Groups  []HelpGroupView
}

type HelpGroupView struct {
	Title string
	Items []HelpItemView
}

type HelpItemView struct {
	Title       string
	Description string
	Usage       string
	Command     string
	Permission  string
}

type SourceView struct {
	Root              string
	PackageSourceType string
	PackageSourceRef  string
	Verified          bool
}

type TrustView struct {
	Level string
	Label string
}

type SummaryView struct {
	ID               string
	Name             string
	Version          string
	Description      string
	Role             string
	State            string
	StateDiagnosis   *StateDiagnosis
	Source           SourceView
	Trust            TrustView
	Commands         []CommandView
	Help             *HelpView
	CommandConflicts []string
}

func BuildSummaryView(snapshot Snapshot, conflicts []string) SummaryView {
	role := summaryViewRole(snapshot)
	state, diagnosis := ProjectState(snapshot)
	return SummaryView{
		ID:               snapshot.PluginID,
		Name:             summaryViewDisplayName(snapshot),
		Version:          strings.TrimSpace(snapshot.Version),
		Description:      strings.TrimSpace(snapshot.Description),
		Role:             role,
		State:            state,
		StateDiagnosis:   diagnosis,
		Source:           buildSourceView(snapshot),
		Trust:            buildTrustView(role, snapshot),
		Commands:         buildCommandViews(snapshot),
		Help:             buildHelpView(snapshot),
		CommandConflicts: normalizeConflictViews(conflicts),
	}
}

func DetectCommandConflicts(snapshots []Snapshot) map[string][]string {
	owners := make(map[string]map[string]struct{})
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		seen := make(map[string]struct{})
		for _, command := range snapshot.Commands {
			if strings.TrimSpace(command.MatchPattern) != "" {
				continue
			}
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

func BuildHelpView(snapshot Snapshot) *HelpView {
	return buildHelpView(snapshot)
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

func buildSourceView(snapshot Snapshot) SourceView {
	root := snapshot.SourceRoot
	if root == "" && len(snapshot.SourceRoots) > 0 {
		root = snapshot.SourceRoots[0]
	}
	return SourceView{
		Root:              root,
		PackageSourceType: snapshot.PackageSourceType,
		PackageSourceRef:  snapshot.PackageSourceRef,
		Verified:          isVerifiedSourceView(snapshot),
	}
}

func isVerifiedSourceView(snapshot Snapshot) bool {
	switch snapshot.SourceRoot {
	case "plugins/builtin", "examples/plugins", "plugins/dev":
		return true
	default:
		return false
	}
}

func buildTrustView(role string, snapshot Snapshot) TrustView {
	switch role {
	case "builtin":
		return TrustView{Level: "official", Label: "官方"}
	case "dev":
		return TrustView{Level: "development", Label: "开发中"}
	case "example":
		return TrustView{Level: "third_party", Label: "示例"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "remote_url" {
			return TrustView{Level: "unverified", Label: "未验证来源"}
		}
		return TrustView{Level: "third_party", Label: "第三方"}
	}
}

func summaryViewDisplayName(snapshot Snapshot) string {
	if strings.TrimSpace(snapshot.Name) != "" {
		return snapshot.Name
	}
	return snapshot.PluginID
}

func summaryViewRole(snapshot Snapshot) string {
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
