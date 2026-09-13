package releaseupdate

import (
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/contractversions"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

const (
	ReleaseRepositoryURL        = "https://github.com/RayleaBot/RayleaBot"
	ManifestAssetName           = "release_manifest.v2.json"
	MaxManifestBytes      int64 = 4 << 20
	PluginManifestVersion       = contractversions.PluginManifestVersion
	PluginUIBridgeVersion       = contractversions.PluginUIBridgeVersion
)

const (
	CodeManifestInvalid = errorcodes.ReleaseManifestInvalid
)

type Error struct {
	Code string
	Op   string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Op, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func errorWithCode(code, op string, err error) error {
	if err == nil {
		err = errors.New("operation failed")
	}
	return &Error{Code: code, Op: op, Err: err}
}

func CodeOf(err error) string {
	var updateErr *Error
	if errors.As(err, &updateErr) {
		return updateErr.Code
	}
	return ""
}

type MatrixVersions struct {
	TransportMatrixVersion         string `json:"transport_matrix_version"`
	CompatibilityMatrixVersion     string `json:"compatibility_matrix_version"`
	ProviderExtensionMatrixVersion string `json:"provider_extension_matrix_version"`
}

type Artifact struct {
	ArtifactID        string `json:"artifact_id"`
	FileName          string `json:"file_name"`
	Platform          string `json:"platform"`
	ArchiveSizeBytes  int64  `json:"archive_size_bytes"`
	ExpandedSizeBytes int64  `json:"expanded_size_bytes"`
	FileCount         int    `json:"file_count"`
	UpdateMode        string `json:"update_mode"`
	SupportLevel      string `json:"support_level"`
	SmokeProfile      string `json:"smoke_profile"`
}

type Manifest struct {
	ManifestVersion       int             `json:"manifest_version"`
	Version               string          `json:"version"`
	GitCommit             string          `json:"git_commit"`
	BuiltAt               string          `json:"built_at"`
	Channel               string          `json:"channel"`
	PublishedAt           string          `json:"published_at"`
	ConfigSchemaVersion   string          `json:"config_schema_version"`
	DBSchemaVersion       string          `json:"db_schema_version"`
	PluginProtocolVersion string          `json:"plugin_protocol_version"`
	PluginManifestVersion string          `json:"plugin_manifest_version"`
	PluginUIBridgeVersion string          `json:"plugin_ui_bridge_version"`
	OneBotMatrix          *MatrixVersions `json:"onebot_matrix,omitempty"`
	Artifacts             []Artifact      `json:"artifacts"`
	ReleaseNotesRef       string          `json:"release_notes_ref"`
}

type BuildInfo struct {
	Version               string          `json:"version"`
	GitCommit             string          `json:"git_commit"`
	ArtifactID            string          `json:"artifact_id"`
	BuiltAt               string          `json:"built_at"`
	PluginManifestVersion string          `json:"plugin_manifest_version"`
	PluginUIBridgeVersion string          `json:"plugin_ui_bridge_version"`
	ReleaseNotesRef       string          `json:"release_notes_ref,omitempty"`
	OneBotMatrix          *MatrixVersions `json:"onebot_matrix,omitempty"`
}

type CheckResult struct {
	Status           string   `json:"status"`
	CurrentVersion   string   `json:"current_version"`
	AvailableVersion string   `json:"available_version,omitempty"`
	UpdateMode       string   `json:"update_mode"`
	ReleasePageURL   string   `json:"release_page_url,omitempty"`
	Artifact         Artifact `json:"artifact"`
}
