package releaseupdate

import (
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

const (
	ReleaseRepositoryURL       = "https://github.com/RayleaBot/RayleaBot"
	ManifestAssetName          = "release_manifest.v2.json"
	MaxManifestBytes     int64 = 4 << 20
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

// Artifact, Manifest and BuildInfo decode only the release fields this package
// reads; the contract defines the full documents.
type Artifact struct {
	ArtifactID       string `json:"artifact_id"`
	FileName         string `json:"file_name"`
	ArchiveSizeBytes int64  `json:"archive_size_bytes"`
	UpdateMode       string `json:"update_mode"`
}

type Manifest struct {
	Version         string     `json:"version"`
	Artifacts       []Artifact `json:"artifacts"`
	ReleaseNotesRef string     `json:"release_notes_ref"`
}

type BuildInfo struct {
	Version    string `json:"version"`
	GitCommit  string `json:"git_commit"`
	ArtifactID string `json:"artifact_id"`
}

type CheckResult struct {
	Status           string   `json:"status"`
	CurrentVersion   string   `json:"current_version"`
	AvailableVersion string   `json:"available_version,omitempty"`
	UpdateMode       string   `json:"update_mode"`
	ReleasePageURL   string   `json:"release_page_url,omitempty"`
	Artifact         Artifact `json:"artifact"`
}
