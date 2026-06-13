package plugins

import (
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
	Version           string
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
		Version:           strings.TrimSpace(snapshot.Version),
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
