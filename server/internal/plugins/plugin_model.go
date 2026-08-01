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
}

type Command struct {
	Name          string
	Aliases       []string
	MatchPattern  string
	Description   string
	Usage         string
	Permission    string
	CommandSource string
	DeclarationID string
}

type CommandPatternDecl struct {
	ID          string
	Name        string
	Pattern     string
	Description string
	Usage       string
	Permission  string
}

type DynamicCommandDecl struct {
	ID          string
	SettingsKey string
	Description string
	UsageArgs   string
	Permission  string
}

type WebhookScope struct {
	Route           string   `json:"route"`
	AuthStrategy    string   `json:"auth_strategy"`
	Header          string   `json:"header"`
	SecretRef       string   `json:"secret_ref"`
	SignaturePrefix string   `json:"signature_prefix,omitempty"`
	SourceIPs       []string `json:"source_ips,omitempty"`
}

type Screenshot struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUI struct {
	Pages []ManagementUIPage `json:"pages"`
}

type ManagementUIPage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Entry string `json:"entry"`
}

type RenderTemplate struct {
	Path string `json:"path"`
}

type Help struct {
	Title   string
	Summary string
	Groups  []HelpGroup
}

type HelpGroup struct {
	Title string
	Items []HelpItem
}

type HelpItem struct {
	Title       string
	Description string
	Usage       string
	Command     string
	Permission  string
}

type Snapshot struct {
	PluginID                string
	Name                    string
	Role                    string
	Version                 string
	Author                  string
	License                 string
	ManifestVersion         string
	PluginProtocolVersion   string
	MinCoreVersion          string
	DataSchemaVersion       string
	Concurrency             int
	Platforms               []string
	Runtime                 string
	Entry                   string
	Description             string
	Icon                    string
	Repo                    string
	Homepage                string
	Keywords                []string
	Screenshots             []Screenshot
	ManagementUI            *ManagementUI
	RenderTemplates         []RenderTemplate
	Help                    *Help
	ArtifactVersion         string
	ArtifactTargetPlatform  string
	ArtifactManifestSHA256  string
	ArtifactBackendSHA256   string
	ArtifactFileCount       int
	ArtifactUIAvailable     bool
	DefaultConfig           map[string]any
	ManifestPath            string
	PackageRootPath         string
	SourceRoot              string
	SourceRoots             []string
	PackageSourceType       string
	PackageSourceRef        string
	Valid                   bool
	ValidationSummary       string
	RegistrationState       string
	DesiredState            string
	RuntimeState            string
	DisplayState            string
	DeadLetter              *DeadLetterSnapshot
	ConflictPaths           []string
	DeclaredCapabilities    []string
	ScopeHTTPHosts          []string
	ScopeStorageRoots       []string
	ScopeThirdPartyAccounts []string
	ScopeWebhooks           []WebhookScope
	Commands                []Command
	ManifestCommands        []Command
	CommandPatterns         []CommandPatternDecl
	DynamicCommands         []DynamicCommandDecl
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
	PluginID     string
	SourceType   string
	SourceRef    string
	Version      string
	ManifestHash string
	PackageHash  string
	InstalledAt  time.Time
}

type PackageRepository interface {
	SavePackageMetadata(context.Context, PackageMetadata) error
	DeletePackageMetadata(context.Context, string) error
}

type PackageMetadataLoader interface {
	LoadAllPackageMetadata(context.Context) (map[string]PackageMetadata, error)
}

type InstallRequest struct {
	SourceType           string
	Source               string
	InspectionID         string
	PackageSHA256        string
	TrustedCodeConfirmed bool
}

type InstallBackendInspection struct {
	Entry  string
	Path   string
	Size   int64
	SHA256 string
}

type InstallUIInspection struct {
	Enabled   bool
	Entry     string
	FileCount int
}

type ArtifactInspection struct {
	Valid          bool
	Version        string
	ManifestSHA256 string
	FileCount      int
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
	Capabilities   []string
	TargetPlatform string
	Backend        InstallBackendInspection
	UI             InstallUIInspection
	Artifact       ArtifactInspection
}

type InstallInspector interface {
	Inspect(context.Context, InstallRequest) (InstallInspection, error)
}

type InstallCoordinator interface {
	Accept(context.Context, InstallRequest) (string, error)
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
	cloned.Platforms = append([]string(nil), snapshot.Platforms...)
	cloned.Keywords = append([]string(nil), snapshot.Keywords...)
	cloned.DeclaredCapabilities = append([]string(nil), snapshot.DeclaredCapabilities...)
	cloned.ScopeHTTPHosts = append([]string(nil), snapshot.ScopeHTTPHosts...)
	cloned.ScopeStorageRoots = append([]string(nil), snapshot.ScopeStorageRoots...)
	cloned.ScopeThirdPartyAccounts = append([]string(nil), snapshot.ScopeThirdPartyAccounts...)
	if len(snapshot.ScopeWebhooks) > 0 {
		cloned.ScopeWebhooks = make([]WebhookScope, 0, len(snapshot.ScopeWebhooks))
		for _, scope := range snapshot.ScopeWebhooks {
			copied := scope
			copied.SourceIPs = append([]string(nil), scope.SourceIPs...)
			cloned.ScopeWebhooks = append(cloned.ScopeWebhooks, copied)
		}
	}
	if len(snapshot.Screenshots) > 0 {
		cloned.Screenshots = make([]Screenshot, 0, len(snapshot.Screenshots))
		for _, screenshot := range snapshot.Screenshots {
			cloned.Screenshots = append(cloned.Screenshots, screenshot)
		}
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
		cloned.Commands = cloneCommands(snapshot.Commands)
	}
	if len(snapshot.ManifestCommands) > 0 {
		cloned.ManifestCommands = cloneCommands(snapshot.ManifestCommands)
	}
	if len(snapshot.CommandPatterns) > 0 {
		cloned.CommandPatterns = append([]CommandPatternDecl(nil), snapshot.CommandPatterns...)
	}
	if len(snapshot.DynamicCommands) > 0 {
		cloned.DynamicCommands = append([]DynamicCommandDecl(nil), snapshot.DynamicCommands...)
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
	if len(help.Groups) > 0 {
		cloned.Groups = make([]HelpGroup, 0, len(help.Groups))
		for _, group := range help.Groups {
			copied := group
			if len(group.Items) > 0 {
				copied.Items = append([]HelpItem(nil), group.Items...)
			}
			cloned.Groups = append(cloned.Groups, copied)
		}
	}
	return &cloned
}

func cloneCommands(commands []Command) []Command {
	if len(commands) == 0 {
		return nil
	}
	cloned := make([]Command, 0, len(commands))
	for _, cmd := range commands {
		copied := cmd
		copied.Aliases = append([]string(nil), cmd.Aliases...)
		cloned = append(cloned, copied)
	}
	return cloned
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
