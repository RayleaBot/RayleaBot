package desktop

// JSON document payloads below are defined by contracts/web-api.openapi.yaml.
// Keeping them opaque here prevents the launcher from creating a second copy of
// the server contract while preserving the exact response sent to the renderer.
type JSONObject map[string]any

type LauncherAdvancedOverrides struct {
	ServerExecutablePath string `json:"serverExecutablePath,omitempty"`
	ConfigPath           string `json:"configPath,omitempty"`
	Workdir              string `json:"workdir,omitempty"`
}

type LauncherSettings struct {
	InstallationRoot  string                     `json:"installationRoot"`
	CloseBehavior     string                     `json:"closeBehavior"`
	AdvancedOverrides *LauncherAdvancedOverrides `json:"advancedOverrides,omitempty"`
}

type LauncherCloseConfirmResponse struct {
	Action       string `json:"action"`
	SetAsDefault bool   `json:"setAsDefault"`
}

type LauncherResolvedSettings struct {
	InstallationRoot     string `json:"installationRoot"`
	ServerExecutablePath string `json:"serverExecutablePath"`
	ConfigPath           string `json:"configPath"`
	Workdir              string `json:"workdir"`
}

type ServerEndpoint struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	BaseURL string `json:"baseUrl"`
}

type EnvironmentCheckResult struct {
	Scope       string `json:"scope"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Summary     string `json:"summary"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation"`
}

type EnvironmentInspection struct {
	Checks                 []EnvironmentCheckResult `json:"checks"`
	PreflightChecks        []EnvironmentCheckResult `json:"preflightChecks"`
	AdvisoryChecks         []EnvironmentCheckResult `json:"advisoryChecks"`
	HasBlockingIssues      bool                     `json:"hasBlockingIssues"`
	CanBootstrapUserConfig bool                     `json:"canBootstrapUserConfig"`
}

type ReleaseCheckSnapshot struct {
	Status           string   `json:"status"`
	CurrentVersion   string   `json:"currentVersion"`
	LatestVersion    string   `json:"latestVersion"`
	Summary          string   `json:"summary"`
	Detail           string   `json:"detail"`
	ErrorCode        string   `json:"errorCode"`
	ReleasePageURL   string   `json:"releasePageUrl"`
	UpdateAvailable  bool     `json:"updateAvailable"`
	DownloadProgress *float64 `json:"downloadProgress"`
	DownloadedBytes  *int64   `json:"downloadedBytes"`
	TotalBytes       *int64   `json:"totalBytes"`
	ArtifactFileName string   `json:"artifactFileName"`
	CanCheck         bool     `json:"canCheck"`
	CanDownload      bool     `json:"canDownload"`
	CanInstall       bool     `json:"canInstall"`
}

type RuntimePrepareResourceProgress struct {
	Kind             string   `json:"kind"`
	Label            string   `json:"label"`
	ResourceID       string   `json:"resourceId"`
	Version          string   `json:"version"`
	SourceLabel      string   `json:"sourceLabel"`
	SourceURL        string   `json:"sourceUrl"`
	ArchivePath      string   `json:"archivePath"`
	StoreRoot        string   `json:"storeRoot"`
	Stage            string   `json:"stage"`
	Status           string   `json:"status"`
	Progress         *float64 `json:"progress"`
	DownloadedBytes  *int64   `json:"downloadedBytes"`
	TotalBytes       *int64   `json:"totalBytes"`
	ExtractedEntries *int64   `json:"extractedEntries"`
	TotalEntries     *int64   `json:"totalEntries"`
	Summary          string   `json:"summary"`
	Error            string   `json:"error"`
	UpdatedAt        string   `json:"updatedAt"`
}

type RuntimePrepareSnapshot struct {
	Active      bool                             `json:"active"`
	CurrentKind string                           `json:"currentKind"`
	Summary     string                           `json:"summary"`
	Resources   []RuntimePrepareResourceProgress `json:"resources"`
}

type LauncherServerSnapshot struct {
	Health       any `json:"health"`
	Readiness    any `json:"readiness"`
	SystemStatus any `json:"systemStatus"`
}

type LauncherLocalSnapshot struct {
	ProcessID            *int64                   `json:"processId"`
	ProcessLifecycle     string                   `json:"processLifecycle"`
	ProcessOwnership     string                   `json:"processOwnership"`
	EnvironmentChecks    []EnvironmentCheckResult `json:"environmentChecks"`
	PreflightChecks      []EnvironmentCheckResult `json:"preflightChecks"`
	AdvisoryChecks       []EnvironmentCheckResult `json:"advisoryChecks"`
	RecentStderr         []string                 `json:"recentStderr"`
	RuntimePrepare       *RuntimePrepareSnapshot  `json:"runtimePrepare"`
	ReleaseCheck         ReleaseCheckSnapshot     `json:"releaseCheck"`
	LastLocalError       string                   `json:"lastLocalError"`
	StatusHint           string                   `json:"statusHint"`
	Settings             LauncherSettings         `json:"settings"`
	ResolvedSettings     LauncherResolvedSettings `json:"resolvedSettings"`
	Endpoint             ServerEndpoint           `json:"endpoint"`
	LocalRecoverySummary any                      `json:"localRecoverySummary"`
}

type LauncherSnapshot struct {
	Server   LauncherServerSnapshot `json:"server"`
	Launcher LauncherLocalSnapshot  `json:"launcher"`
}

type TrayMenuState struct {
	TrayStatusSummary       string `json:"trayStatusSummary"`
	CanOpenWebUI            bool   `json:"canOpenWebUi"`
	TrayServiceAction       string `json:"trayServiceAction"`
	TrayServiceActionLabel  string `json:"trayServiceActionLabel"`
	CanRunTrayServiceAction bool   `json:"canRunTrayServiceAction"`
}

func releaseUnavailable(detail string) ReleaseCheckSnapshot {
	return ReleaseCheckSnapshot{
		Status:  "disabled",
		Summary: "版本信息不可用",
		Detail:  detail,
	}
}
