package plugins

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInstallInspectionRequired = errors.New("plugin install inspection required")
	ErrInstallInspectionExpired  = errors.New("plugin install inspection expired")
	ErrInstallDigestMismatch     = errors.New("plugin install digest mismatch")
	ErrTrustedCodeConfirmation   = errors.New("trusted code confirmation required")
)

type CatalogView interface {
	List() []Snapshot
	Get(string) (Snapshot, bool)
	SetDesiredState(string, string) (Snapshot, error)
}

type CatalogStore interface {
	List() []Snapshot
	Get(string) (Snapshot, bool)
	Replace([]Snapshot)
	RefreshInstalled([]Snapshot, string)
}

type Command struct {
	ID           string
	Name         string
	DisplayName  string
	Aliases      []string
	TriggerType  string
	TriggerNames []string
	MatchPattern string
	SettingsKey  string
	Description  string
	Usage        string
	Permission   string
}

type CommandGroup struct {
	ID       string
	Title    string
	Commands []string
}

type PermissionGrant struct {
	Platforms []string `json:"platforms,omitempty"`
}

type WebhookScope struct {
	ID               string                  `json:"id"`
	Route            string                  `json:"route"`
	AuthStrategy     string                  `json:"auth_strategy"`
	Header           string                  `json:"header"`
	SecretRef        string                  `json:"secret_ref"`
	SignaturePrefix  string                  `json:"signature_prefix,omitempty"`
	SourceCIDRs      []string                `json:"source_cidrs,omitempty"`
	MaxBodyBytes     int                     `json:"max_body_bytes,omitempty"`
	ReplayProtection WebhookReplayProtection `json:"replay_protection"`
}

type WebhookReplayProtection struct {
	TimestampHeader  string `json:"timestamp_header"`
	EventIDHeader    string `json:"event_id_header"`
	ToleranceSeconds int    `json:"tolerance_seconds"`
	Enforce          bool   `json:"enforce"`
}

type Screenshot struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUI struct {
	Entry string             `json:"entry"`
	Pages []ManagementUIPage `json:"pages"`
}

type ManagementUIPage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type RenderTemplate struct {
	Path string `json:"path"`
}

type Help struct {
	Title   string
	Summary string
}

type Snapshot struct {
	PluginID               string
	Name                   string
	Role                   string
	Version                string
	Author                 string
	License                string
	ManifestVersion        string
	MinCoreVersion         string
	Concurrency            int
	Events                 []string
	Permissions            map[string]PermissionGrant
	Webhooks               []WebhookScope
	CommandGroups          []CommandGroup
	Description            string
	Icon                   string
	Repo                   string
	Homepage               string
	Keywords               []string
	Screenshots            []Screenshot
	ManagementUI           *ManagementUI
	RenderTemplates        []RenderTemplate
	Help                   *Help
	ArtifactVersion        string
	ArtifactTargetPlatform string
	ArtifactUIAvailable    bool
	DefaultConfig          map[string]any
	ManifestPath           string
	PackageRootPath        string
	SourceRoot             string
	SourceRoots            []string
	PackageSourceType      string
	PackageSourceRef       string
	Valid                  bool
	ValidationSummary      string
	RegistrationState      string
	DesiredState           string
	RuntimeState           string
	DisplayState           string
	DeadLetter             *DeadLetterSnapshot
	ConflictPaths          []string
	Commands               []Command
	ManifestCommands       []Command
}

// DeadLetterSnapshot captures the context recorded when a plugin runtime
// exhausted its crash-restart budget. The catalog only stores this object
// while runtime_state equals dead_letter; SetRuntimeState into any other
// state clears it so management surfaces never show stale dwell-time.
type DeadLetterSnapshot struct {
	EnteredAt        time.Time
	CrashCount       int
	LastErrorCode    string
	LastErrorMessage string
}

type DesiredStateRepository interface {
	LoadDesiredStates(context.Context) (map[string]string, error)
	SaveDesiredState(context.Context, string, string, time.Time) error
	DeleteDesiredState(context.Context, string) error
}

type PackageMetadata struct {
	PluginID    string
	SourceType  string
	SourceRef   string
	Version     string
	PackageHash string
	InstalledAt time.Time
}

type PackageRepository interface {
	SavePackageMetadata(context.Context, PackageMetadata) error
	DeletePackageMetadata(context.Context, string) error
}

type PackageMetadataLoader interface {
	LoadAllPackageMetadata(context.Context) (map[string]PackageMetadata, error)
}

type InstallRequest struct {
	SourceType            string
	Source                string
	SourceLabel           string
	ResolvedSourceType    string
	ResolvedSource        string
	ExpectedArchiveSHA256 string
	ReplaceExisting       bool
	TrustedCodeRequired   bool
}

type InstallBackendInspection struct {
	Entry string
	Path  string
	Size  int64
}

type InstallUIInspection struct {
	Enabled   bool
	Entry     string
	FileCount int
}

type ArtifactInspection struct {
	Valid     bool
	Version   string
	FileCount int
}

type InstallInspection struct {
	InspectionID   string
	ExpiresAt      time.Time
	PackageSHA256  string
	SourceType     string
	Source         string
	PluginID       string
	PluginName     string
	Version        string
	Author         string
	License        string
	SourceLabel    string
	Permissions    map[string]PermissionGrant
	TargetPlatform string
	Backend        InstallBackendInspection
	UI             InstallUIInspection
	Artifact       ArtifactInspection
}

type InstallInspector interface {
	Inspect(context.Context, InstallRequest) (InstallInspection, error)
}

type InstallAcceptance struct {
	InspectionID         string
	PackageSHA256        string
	TrustedCodeConfirmed bool
}

type InstallCoordinator interface {
	Accept(context.Context, InstallAcceptance) (string, error)
	Cancel(string) bool
	Close() error
}

type StopPluginFunc func(context.Context, string)

type UninstallCoordinator interface {
	Accept(ctx context.Context, pluginID string) (string, error)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.DisplayState = projectDisplayState(snapshot)
	cloned.DefaultConfig = cloneMap(snapshot.DefaultConfig)
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	cloned.Events = append([]string(nil), snapshot.Events...)
	cloned.Keywords = append([]string(nil), snapshot.Keywords...)
	if len(snapshot.Permissions) > 0 {
		cloned.Permissions = make(map[string]PermissionGrant, len(snapshot.Permissions))
		for name, grant := range snapshot.Permissions {
			grant.Platforms = append([]string(nil), grant.Platforms...)
			cloned.Permissions[name] = grant
		}
	}
	if len(snapshot.Webhooks) > 0 {
		cloned.Webhooks = cloneWebhookScopes(snapshot.Webhooks)
	}
	if len(snapshot.CommandGroups) > 0 {
		cloned.CommandGroups = make([]CommandGroup, 0, len(snapshot.CommandGroups))
		for _, group := range snapshot.CommandGroups {
			group.Commands = append([]string(nil), group.Commands...)
			cloned.CommandGroups = append(cloned.CommandGroups, group)
		}
	}
	if len(snapshot.Screenshots) > 0 {
		cloned.Screenshots = make([]Screenshot, 0, len(snapshot.Screenshots))
		cloned.Screenshots = append(cloned.Screenshots, snapshot.Screenshots...)
	}
	if snapshot.ManagementUI != nil {
		copied := *snapshot.ManagementUI
		copied.Pages = append([]ManagementUIPage(nil), snapshot.ManagementUI.Pages...)
		cloned.ManagementUI = &copied
	}
	if len(snapshot.RenderTemplates) > 0 {
		cloned.RenderTemplates = append([]RenderTemplate(nil), snapshot.RenderTemplates...)
	}
	if snapshot.Help != nil {
		cloned.Help = cloneHelp(snapshot.Help)
	}
	if snapshot.DeadLetter != nil {
		copied := *snapshot.DeadLetter
		cloned.DeadLetter = &copied
	}
	if len(snapshot.Commands) > 0 {
		cloned.Commands = CloneCommands(snapshot.Commands)
	}
	if len(snapshot.ManifestCommands) > 0 {
		cloned.ManifestCommands = CloneCommands(snapshot.ManifestCommands)
	}
	return cloned
}

func CloneSnapshot(snapshot Snapshot) Snapshot {
	return cloneSnapshot(snapshot)
}

func CloneSettings(values map[string]any) map[string]any {
	cloned := cloneMap(values)
	if cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func ClonePermissions(values map[string]PermissionGrant) map[string]PermissionGrant {
	if len(values) == 0 {
		return map[string]PermissionGrant{}
	}
	cloned := make(map[string]PermissionGrant, len(values))
	for name, grant := range values {
		grant.Platforms = append([]string(nil), grant.Platforms...)
		cloned[name] = grant
	}
	return cloned
}

func CloneSettingValue(value any) any {
	return cloneValue(value)
}

func ApplyPackageMetadata(entries []Snapshot, metadata map[string]PackageMetadata) []Snapshot {
	if len(entries) == 0 {
		return nil
	}

	enriched := make([]Snapshot, 0, len(entries))
	for _, entry := range entries {
		cloned := cloneSnapshot(entry)
		if pkg, ok := metadata[cloned.PluginID]; ok {
			cloned.PackageSourceType = pkg.SourceType
			cloned.PackageSourceRef = pkg.SourceRef
		}
		enriched = append(enriched, cloned)
	}
	return enriched
}

func cloneHelp(help *Help) *Help {
	if help == nil {
		return nil
	}
	cloned := *help
	return &cloned
}

// CloneCommands isolates all mutable command declaration fields.
func CloneCommands(commands []Command) []Command {
	if len(commands) == 0 {
		return nil
	}
	cloned := make([]Command, 0, len(commands))
	for _, cmd := range commands {
		copied := cmd
		copied.Aliases = append([]string(nil), cmd.Aliases...)
		copied.TriggerNames = append([]string(nil), cmd.TriggerNames...)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneWebhookScopes(scopes []WebhookScope) []WebhookScope {
	items := make([]WebhookScope, 0, len(scopes))
	for _, scope := range scopes {
		copied := scope
		copied.SourceCIDRs = append([]string(nil), scope.SourceCIDRs...)
		items = append(items, copied)
	}
	return items
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func CloneMap(values map[string]any) map[string]any {
	return cloneMap(values)
}

func cloneSlice(values []any) []any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]any, len(values))
	for i, value := range values {
		cloned[i] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}
