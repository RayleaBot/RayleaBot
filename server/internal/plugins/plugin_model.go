package plugins

import (
	"context"
	"time"
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
	SDKMinVersion           string
	RuntimeVersion          string
	MinCoreVersion          string
	DataSchemaVersion       string
	Concurrency             int
	Platforms               []string
	Runtime                 string
	Entry                   string
	Type                    string
	Description             string
	Icon                    string
	Repo                    string
	Homepage                string
	Keywords                []string
	Screenshots             []Screenshot
	ManagementUI            *ManagementUI
	RenderTemplates         []RenderTemplate
	Help                    *Help
	SystemDependencies      []string
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
	PythonDependencies      []string
	NodeDependencies        []string
	RequireInstallScripts   bool
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
