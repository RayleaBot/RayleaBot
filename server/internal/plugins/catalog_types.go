package plugins

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrPluginNotFound        = errors.New("plugin not found")
	ErrStateConflict         = errors.New("state conflict")
	ErrPluginNotInDeadLetter = errors.New("plugin is not in dead_letter")
)

type PermissionPendingError struct {
	PluginID            string
	MissingCapabilities []string
	ScopeChanged        bool
}

func (e *PermissionPendingError) Error() string {
	return "plugin permission pending"
}

type Command struct {
	Name          string
	Aliases       []string
	Description   string
	Usage         string
	Permission    string
	CommandSource string
	DeclarationID string
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
	PluginID              string
	Name                  string
	Role                  string
	Version               string
	Author                string
	License               string
	SDKMinVersion         string
	RuntimeVersion        string
	MinCoreVersion        string
	DataSchemaVersion     string
	Concurrency           int
	Platforms             []string
	Runtime               string
	Entry                 string
	Type                  string
	Description           string
	Icon                  string
	Repo                  string
	Homepage              string
	Keywords              []string
	Screenshots           []Screenshot
	ManagementUI          *ManagementUI
	RenderTemplates       []RenderTemplate
	Help                  *Help
	SystemDependencies    []string
	DefaultConfig         map[string]any
	ManifestPath          string
	PackageRootPath       string
	SourceRoot            string
	SourceRoots           []string
	PackageSourceType     string
	PackageSourceRef      string
	Valid                 bool
	ValidationSummary     string
	RegistrationState     string
	DesiredState          string
	RuntimeState          string
	DisplayState          string
	DeadLetter            *DeadLetterSnapshot
	ConflictPaths         []string
	RequiredPermissions   []string
	OptionalPermissions   []string
	DeclaredCapabilities  []string
	PythonDependencies    []string
	NodeDependencies      []string
	RequireInstallScripts bool
	ScopeHTTPHosts        []string
	ScopeStorageRoots     []string
	ScopeWebhooks         []WebhookScope
	Commands              []Command
	ManifestCommands      []Command
	DynamicCommands       []DynamicCommandDecl
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

type Catalog struct {
	mu          sync.RWMutex
	order       []string
	items       map[string]Snapshot
	nextSubID   uint64
	subscribers map[uint64]chan Snapshot
}
