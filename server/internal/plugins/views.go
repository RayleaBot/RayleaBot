package plugins

import (
	"sort"
	"strings"
	"time"
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
	ID                string
	Name              string
	Description       string
	Role              string
	RegistrationState string
	DesiredState      string
	RuntimeState      string
	DisplayState      string
	Source            SourceView
	Trust             TrustView
	Commands          []CommandView
	Help              *HelpView
	CommandConflicts  []string
}

type GrantSource string

const (
	GrantSourceBuiltinAuto GrantSource = "builtin_auto"
	GrantSourceConfigAuto  GrantSource = "config_auto"
	GrantSourcePersisted   GrantSource = "persisted"
)

type PermissionRequirement string

const (
	PermissionRequirementRequired PermissionRequirement = "required"
	PermissionRequirementOptional PermissionRequirement = "optional"
)

type PermissionStatus string

const (
	PermissionStatusGranted    PermissionStatus = "granted"
	PermissionStatusNotGranted PermissionStatus = "not_granted"
)

type PermissionSource string

const (
	PermissionSourceBuiltinAuto PermissionSource = "builtin_auto"
	PermissionSourceConfigAuto  PermissionSource = "config_auto"
	PermissionSourcePersisted   PermissionSource = "persisted"
	PermissionSourceNone        PermissionSource = "none"
)

type EffectiveGrant struct {
	PluginID   string
	Capability string
	GrantedAt  *time.Time
	ExpiresAt  *time.Time
	Source     GrantSource
	ScopeJSON  string
}

type PermissionSummary struct {
	Capability  string
	Requirement PermissionRequirement
	Status      PermissionStatus
	Source      PermissionSource
	ExpiresAt   *time.Time
}

func BuildSummaryView(snapshot Snapshot, conflicts []string) SummaryView {
	role := summaryViewRole(snapshot)
	return SummaryView{
		ID:                snapshot.PluginID,
		Name:              summaryViewDisplayName(snapshot),
		Description:       strings.TrimSpace(snapshot.Description),
		Role:              role,
		RegistrationState: snapshot.RegistrationState,
		DesiredState:      snapshot.DesiredState,
		RuntimeState:      snapshot.RuntimeState,
		DisplayState:      snapshot.DisplayState,
		Source:            buildSourceView(snapshot),
		Trust:             buildTrustView(role, snapshot),
		Commands:          buildCommandViews(snapshot),
		Help:              buildHelpView(snapshot),
		CommandConflicts:  normalizeConflictViews(conflicts),
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

func ComputeEffectiveGrants(snapshot Snapshot, configAutoCapabilities []string, persisted []PluginGrant) []EffectiveGrant {
	items := make(map[string]EffectiveGrant)
	scopeJSON := BuildScopeJSON(snapshot)

	for _, capability := range builtinAutoCapabilities(snapshot) {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   snapshot.PluginID,
			Capability: capability,
			Source:     GrantSourceBuiltinAuto,
			ScopeJSON:  scopeJSON,
		})
	}

	for _, capability := range dedupeCapabilities(configAutoCapabilities) {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   snapshot.PluginID,
			Capability: capability,
			Source:     GrantSourceConfigAuto,
			ScopeJSON:  scopeJSON,
		})
	}

	for _, grant := range persisted {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   grant.PluginID,
			Capability: strings.TrimSpace(grant.Capability),
			GrantedAt:  cloneTimePointer(&grant.GrantedAt),
			ExpiresAt:  cloneTimePointer(grant.ExpiresAt),
			Source:     GrantSourcePersisted,
			ScopeJSON:  grant.ScopeJSON,
		})
	}

	effective := make([]EffectiveGrant, 0, len(items))
	for _, item := range items {
		effective = append(effective, item)
	}
	sort.Slice(effective, func(left, right int) bool {
		return effective[left].Capability < effective[right].Capability
	})
	return effective
}

func BuildPermissionSummaries(snapshot Snapshot, effectiveGrants []EffectiveGrant) []PermissionSummary {
	grants := make(map[string]EffectiveGrant, len(effectiveGrants))
	for _, grant := range effectiveGrants {
		grants[grant.Capability] = grant
	}

	summaries := make([]PermissionSummary, 0, len(snapshot.RequiredPermissions)+len(snapshot.OptionalPermissions))
	seen := make(map[string]struct{}, len(snapshot.RequiredPermissions)+len(snapshot.OptionalPermissions))
	for _, capability := range snapshot.RequiredPermissions {
		summaries = appendPermissionSummary(summaries, seen, grants, capability, PermissionRequirementRequired)
	}
	for _, capability := range snapshot.OptionalPermissions {
		summaries = appendPermissionSummary(summaries, seen, grants, capability, PermissionRequirementOptional)
	}
	return summaries
}

func appendPermissionSummary(
	summaries []PermissionSummary,
	seen map[string]struct{},
	grants map[string]EffectiveGrant,
	capability string,
	requirement PermissionRequirement,
) []PermissionSummary {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return summaries
	}
	if _, ok := seen[capability]; ok {
		return summaries
	}
	seen[capability] = struct{}{}

	summary := PermissionSummary{
		Capability:  capability,
		Requirement: requirement,
		Status:      PermissionStatusNotGranted,
		Source:      PermissionSourceNone,
	}
	if grant, ok := grants[capability]; ok {
		summary.Status = PermissionStatusGranted
		summary.Source = grantSourceAsPermissionSource(grant.Source)
		summary.ExpiresAt = cloneTimePointer(grant.ExpiresAt)
	}
	return append(summaries, summary)
}

func grantSourceAsPermissionSource(source GrantSource) PermissionSource {
	switch source {
	case GrantSourceBuiltinAuto:
		return PermissionSourceBuiltinAuto
	case GrantSourceConfigAuto:
		return PermissionSourceConfigAuto
	case GrantSourcePersisted:
		return PermissionSourcePersisted
	default:
		return PermissionSourceNone
	}
}

func putEffectiveGrant(items map[string]EffectiveGrant, grant EffectiveGrant) {
	capability := strings.TrimSpace(grant.Capability)
	if capability == "" {
		return
	}
	grant.Capability = capability
	current, exists := items[capability]
	if !exists || grantSourcePriority(grant.Source) < grantSourcePriority(current.Source) {
		items[capability] = grant
		return
	}
	if current.ScopeJSON == "" && strings.TrimSpace(grant.ScopeJSON) != "" {
		current.ScopeJSON = grant.ScopeJSON
		items[capability] = current
	}
}

func grantSourcePriority(source GrantSource) int {
	switch source {
	case GrantSourceBuiltinAuto:
		return 0
	case GrantSourceConfigAuto:
		return 1
	case GrantSourcePersisted:
		return 2
	default:
		return 99
	}
}

func builtinAutoCapabilities(snapshot Snapshot) []string {
	if summaryViewRole(snapshot) != "builtin" {
		return nil
	}
	return dedupeCapabilities(append(append([]string{}, snapshot.RequiredPermissions...), snapshot.OptionalPermissions...))
}

func dedupeCapabilities(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
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

func addSummaryConflictToken(tokens map[string]struct{}, raw string) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return
	}
	tokens[token] = struct{}{}
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
