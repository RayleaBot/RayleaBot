package plugins

import (
	"sort"
	"strings"
)

type CommandView struct {
	ID            string
	Name          string
	EffectiveName string
	Aliases       []string
	Description   string
	Usage         string
	Permission    string
	TriggerType   string
	TriggerNames  []string
	MatchPattern  string
	SettingsKey   string
}

type HelpView struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type SourceView struct {
	Root              string `json:"root"`
	PackageSourceType string `json:"package_source_type,omitempty"`
	PackageSourceRef  string `json:"package_source_ref,omitempty"`
	Verified          bool   `json:"verified"`
}

type TrustView struct {
	Level string `json:"level"`
	Label string `json:"label"`
}

// Summary is the shared display state. Command triggers and absent-help
// serialization are projected by each public boundary.
type Summary struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Version          string          `json:"version,omitempty"`
	Description      string          `json:"description,omitempty"`
	Author           string          `json:"author,omitempty"`
	Icon             string          `json:"icon,omitempty"`
	Role             string          `json:"role"`
	State            string          `json:"state"`
	StateDiagnosis   *StateDiagnosis `json:"state_diagnosis,omitempty"`
	Source           SourceView      `json:"source"`
	Trust            TrustView       `json:"trust"`
	CommandConflicts []string        `json:"command_conflicts"`
}

type SummaryView struct {
	Summary
	Commands      []CommandView
	CommandGroups []CommandGroup
	Help          *HelpView
}

func BuildSummary(snapshot Snapshot, conflicts []string) Summary {
	role := summaryViewRole(snapshot)
	state, diagnosis := ProjectState(snapshot)
	return Summary{ID: snapshot.PluginID, Name: summaryViewDisplayName(snapshot), Version: strings.TrimSpace(snapshot.Version), Description: strings.TrimSpace(snapshot.Description), Author: strings.TrimSpace(snapshot.Author), Icon: strings.TrimSpace(snapshot.Icon), Role: role, State: state, StateDiagnosis: diagnosis, Source: buildSourceView(snapshot), Trust: buildTrustView(role, snapshot), CommandConflicts: normalizeConflictViews(conflicts)}
}

func BuildSummaryView(snapshot Snapshot, conflicts []string) SummaryView {
	return SummaryView{Summary: BuildSummary(snapshot, conflicts), Commands: buildCommandViews(snapshot), CommandGroups: cloneCommandGroups(snapshot.CommandGroups), Help: buildHelpView(snapshot)}
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
			ID:            strings.TrimSpace(command.ID),
			Name:          strings.TrimSpace(command.DisplayName),
			EffectiveName: strings.TrimSpace(command.Name),
			Aliases:       normalizeStringViews(command.Aliases),
			Description:   strings.TrimSpace(command.Description),
			Usage:         strings.TrimSpace(command.Usage),
			Permission:    strings.TrimSpace(command.Permission),
			TriggerType:   strings.TrimSpace(command.TriggerType),
			TriggerNames:  normalizeStringViews(command.TriggerNames),
			MatchPattern:  strings.TrimSpace(command.MatchPattern),
			SettingsKey:   strings.TrimSpace(command.SettingsKey),
		})
	}
	return items
}

func cloneCommandGroups(groups []CommandGroup) []CommandGroup {
	if len(groups) == 0 {
		return []CommandGroup{}
	}
	result := make([]CommandGroup, 0, len(groups))
	for _, group := range groups {
		group.Commands = append([]string(nil), group.Commands...)
		result = append(result, group)
	}
	return result
}

func buildHelpView(snapshot Snapshot) *HelpView {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.Help == nil {
		return nil
	}

	help := &HelpView{
		Title:   strings.TrimSpace(snapshot.Help.Title),
		Summary: strings.TrimSpace(snapshot.Help.Summary),
	}
	if help.Title == "" && help.Summary == "" {
		return nil
	}
	return help
}

func BuildHelpView(snapshot Snapshot) *HelpView {
	return buildHelpView(snapshot)
}

func EffectiveCommandNames(triggerType, name string, aliases []string) []string {
	if strings.TrimSpace(triggerType) == "pattern" {
		return []string{}
	}
	items := make([]string, 0, 1+len(aliases))
	if name = strings.TrimSpace(name); name != "" {
		items = append(items, name)
	}
	return append(items, aliases...)
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
	switch snapshot.PackageSourceType {
	case "catalog", "development":
		return true
	default:
		return false
	}
}

func buildTrustView(role string, snapshot Snapshot) TrustView {
	switch role {
	case "official":
		return TrustView{Level: "official", Label: "官方"}
	case "development":
		return TrustView{Level: "development", Label: "开发中"}
	default:
		if snapshot.PackageSourceType == "local_zip" || snapshot.PackageSourceType == "local_directory" || snapshot.PackageSourceType == "remote_url" {
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
	if snapshot.PackageSourceType == "catalog" && snapshot.PackageSourceRef == "official" {
		return "official"
	}
	if snapshot.PackageSourceType == "development" {
		return "development"
	}
	return "community"
}
