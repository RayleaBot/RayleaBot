package pluginmarket

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	CodeCatalogUnavailable = "plugin.store_catalog_unavailable"
	CodeReleaseUnavailable = "plugin.store_release_unavailable"
	CodeIntegrityMismatch  = "plugin.store_integrity_mismatch"
)

var (
	ErrCatalogUnavailable = errors.New("plugin store catalog unavailable")
	ErrEntryNotFound      = errors.New("plugin store entry not found")
	ErrReleaseUnavailable = errors.New("plugin store release unavailable")
	ErrIntegrityMismatch  = errors.New("plugin store artifact integrity mismatch")
)

type StoreError struct {
	Code string
	Err  error
}

func (e *StoreError) Error() string {
	if e == nil || e.Err == nil {
		return "plugin store error"
	}
	return e.Err.Error()
}

func (e *StoreError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errorWithCode(code string, err error) error {
	return &StoreError{Code: code, Err: err}
}

func ErrorCode(err error) string {
	var storeErr *StoreError
	if errors.As(err, &storeErr) {
		return storeErr.Code
	}
	return ""
}

type Catalog struct {
	CatalogVersion string  `json:"catalog_version"`
	GeneratedAt    string  `json:"generated_at"`
	Entries        []Entry `json:"entries"`
}

type Entry struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Summary       string    `json:"summary"`
	Description   string    `json:"description,omitempty"`
	Publisher     Publisher `json:"publisher"`
	RepositoryURL string    `json:"repository_url"`
	Homepage      string    `json:"homepage,omitempty"`
	License       string    `json:"license"`
	Keywords      []string  `json:"keywords"`
	Recommended   bool      `json:"recommended"`
	Releases      []Release `json:"releases"`
}

type Publisher struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

type Release struct {
	Version        string  `json:"version"`
	PublishedAt    string  `json:"published_at"`
	MinCoreVersion string  `json:"min_core_version"`
	Yanked         bool    `json:"yanked"`
	Assets         []Asset `json:"assets"`
}

type Asset struct {
	Platform         string `json:"platform"`
	URL              string `json:"url"`
	ArchiveSizeBytes int64  `json:"archive_size_bytes"`
	ArchiveSHA256    string `json:"archive_sha256"`
	ManifestSHA256   string `json:"manifest_sha256"`
}

type SignatureEnvelope struct {
	SignatureVersion int         `json:"signature_version"`
	Algorithm        string      `json:"algorithm"`
	CatalogSHA256    string      `json:"catalog_sha256"`
	KeyID            string      `json:"key_id"`
	Signatures       []Signature `json:"signatures"`
}

type Signature struct {
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

type CatalogStatus struct {
	Source        string    `json:"source"`
	Verified      bool      `json:"verified"`
	GeneratedAt   time.Time `json:"generated_at"`
	EntryCount    int       `json:"entry_count"`
	TrustedKeyIDs []string  `json:"trusted_key_ids,omitempty"`
}

type ReleaseView struct {
	Version        string    `json:"version"`
	PublishedAt    time.Time `json:"published_at"`
	MinCoreVersion string    `json:"min_core_version"`
	Compatible     bool      `json:"compatible"`
	AssetAvailable bool      `json:"asset_available"`
	Yanked         bool      `json:"yanked"`
}

type EntryView struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Summary          string       `json:"summary"`
	Description      string       `json:"description,omitempty"`
	Publisher        Publisher    `json:"publisher"`
	RepositoryURL    string       `json:"repository_url"`
	Homepage         string       `json:"homepage,omitempty"`
	License          string       `json:"license"`
	Keywords         []string     `json:"keywords"`
	Recommended      bool         `json:"recommended"`
	LatestRelease    *ReleaseView `json:"latest_release,omitempty"`
	InstalledVersion string       `json:"installed_version,omitempty"`
	InstallState     string       `json:"install_state"`
}

type Query struct {
	Text      string
	Publisher string
	Sort      string
	Cursor    int
	Limit     int
}

type ListResult struct {
	Items      []EntryView   `json:"items"`
	Total      int           `json:"total"`
	NextCursor string        `json:"next_cursor,omitempty"`
	Catalog    CatalogStatus `json:"catalog"`
}

type DetailResult struct {
	Plugin   EntryView     `json:"plugin"`
	Releases []ReleaseView `json:"releases"`
	Catalog  CatalogStatus `json:"catalog"`
}

type InstallRequest struct {
	PluginID             string
	Version              string
	TrustedCodeConfirmed bool
}

type Installer interface {
	plugins.InstallInspector
	plugins.InstallCoordinator
}

type ServiceAPI interface {
	List(Query) ListResult
	Get(string) (DetailResult, bool)
	Refresh(context.Context) (CatalogStatus, error)
	Install(context.Context, InstallRequest) (string, error)
}

func invalidCatalog(format string, args ...any) error {
	return errorWithCode(CodeCatalogUnavailable, fmt.Errorf(format, args...))
}
