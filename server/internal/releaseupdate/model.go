package releaseupdate

import (
	"errors"
	"fmt"
)

const (
	ProtocolVersion            = 2
	ReleaseRepositoryURL       = "https://github.com/RayleaBot/RayleaBot"
	ManifestAssetName          = "release_manifest.v2.json"
	SignatureAssetName         = "release_manifest.v2.sig.json"
	MaxManifestBytes     int64 = 4 << 20
	MaxArchiveBytes      int64 = 2 << 30
	MaxExpandedBytes     int64 = 8 << 30
	MaxArtifactFiles           = 100_000
	MaxCompressionRatio        = 100
)

const (
	CodeTrustRequired         = "release.trust_required"
	CodeManifestInvalid       = "release.manifest_invalid"
	CodeSignatureInvalid      = "release.signature_invalid"
	CodeManifestExpired       = "release.manifest_expired"
	CodeReplayRejected        = "release.replay_rejected"
	CodeArtifactInvalid       = "release.artifact_invalid"
	CodeUpdateNotSupported    = "release.update_not_supported"
	CodeDiskSpaceInsufficient = "release.disk_space_insufficient"
	CodeInstallFailed         = "release.install_failed"
	CodeRollbackFailed        = "release.rollback_failed"
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
	ArtifactID                string `json:"artifact_id"`
	FileName                  string `json:"file_name"`
	Platform                  string `json:"platform"`
	SHA256                    string `json:"sha256"`
	ArchiveSizeBytes          int64  `json:"archive_size_bytes"`
	ExpandedSizeBytes         int64  `json:"expanded_size_bytes"`
	FileCount                 int    `json:"file_count"`
	UpdateMode                string `json:"update_mode"`
	MinUpdaterProtocolVersion int    `json:"min_updater_protocol_version"`
	WindowsSignerSHA256       string `json:"windows_signer_sha256,omitempty"`
	SupportLevel              string `json:"support_level"`
	DepsManifestSHA256        string `json:"deps_manifest_sha256"`
	SmokeProfile              string `json:"smoke_profile"`
}

type Manifest struct {
	ManifestVersion       int             `json:"manifest_version"`
	Version               string          `json:"version"`
	GitCommit             string          `json:"git_commit"`
	BuiltAt               string          `json:"built_at"`
	Channel               string          `json:"channel"`
	PublishedAt           string          `json:"published_at"`
	ExpiresAt             string          `json:"expires_at"`
	UpdateProtocolVersion int             `json:"update_protocol_version"`
	ConfigSchemaVersion   string          `json:"config_schema_version"`
	DBSchemaVersion       string          `json:"db_schema_version"`
	PluginProtocolVersion string          `json:"plugin_protocol_version"`
	OneBotMatrix          *MatrixVersions `json:"onebot_matrix,omitempty"`
	Artifacts             []Artifact      `json:"artifacts"`
	ReleaseNotesRef       string          `json:"release_notes_ref"`
}

type Signature struct {
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type SignatureEnvelope struct {
	SignatureVersion int         `json:"signature_version"`
	Algorithm        string      `json:"algorithm"`
	ManifestSHA256   string      `json:"manifest_sha256"`
	KeyID            string      `json:"key_id"`
	Signatures       []Signature `json:"signatures"`
}

type BuildInfo struct {
	Version               string          `json:"version"`
	GitCommit             string          `json:"git_commit"`
	ArtifactID            string          `json:"artifact_id"`
	BuiltAt               string          `json:"built_at"`
	UpdateProtocolVersion int             `json:"update_protocol_version"`
	ReleaseNotesRef       string          `json:"release_notes_ref,omitempty"`
	ReleaseManifestSHA256 string          `json:"release_manifest_sha256,omitempty"`
	OneBotMatrix          *MatrixVersions `json:"onebot_matrix,omitempty"`
}

type VerifiedManifest struct {
	Manifest      Manifest
	Envelope      SignatureEnvelope
	ManifestBytes []byte
	Digest        string
	TrustedKeyIDs []string
}

func (v VerifiedManifest) ArtifactByID(artifactID string) (Artifact, bool) {
	for _, artifact := range v.Manifest.Artifacts {
		if artifact.ArtifactID == artifactID {
			return artifact, true
		}
	}
	return Artifact{}, false
}

type CheckResult struct {
	Status           string   `json:"status"`
	CurrentVersion   string   `json:"current_version"`
	AvailableVersion string   `json:"available_version,omitempty"`
	UpdateMode       string   `json:"update_mode"`
	ReleasePageURL   string   `json:"release_page_url,omitempty"`
	AutomaticAllowed bool     `json:"automatic_install_supported"`
	ManifestPath     string   `json:"manifest_path,omitempty"`
	SignaturePath    string   `json:"signature_path,omitempty"`
	Artifact         Artifact `json:"artifact"`
	ManifestDigest   string   `json:"manifest_sha256"`
	TrustedKeyIDs    []string `json:"trusted_key_ids,omitempty"`
}

type DownloadedBundle struct {
	CheckResult
	ArtifactPath string `json:"artifact_path"`
}
