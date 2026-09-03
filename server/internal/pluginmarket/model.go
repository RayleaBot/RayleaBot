package pluginmarket

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

const (
	OfficialSourceID       = "official"
	CodeCatalogUnavailable = "plugin.store_catalog_unavailable"
	CodeReleaseUnavailable = "plugin.store_release_unavailable"
	CodeIntegrityMismatch  = "plugin.store_integrity_mismatch"
)

var (
	ErrCatalogUnavailable = errors.New("plugin store catalog unavailable")
	ErrEntryNotFound      = errors.New("plugin store entry not found")
	ErrReleaseUnavailable = errors.New("plugin store release unavailable")
	ErrIntegrityMismatch  = errors.New("plugin store artifact integrity mismatch")
	ErrSourceNotFound     = errors.New("plugin store source not found")
	ErrSourceImmutable    = errors.New("official plugin store source is immutable")
	ErrSourceConflict     = errors.New("plugin store source already exists")
	ErrSourceInvalid      = errors.New("plugin store source is invalid")
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
	Entries        []Entry `json:"entries"`
}

type Entry struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Summary        string          `json:"summary"`
	Description    string          `json:"description,omitempty"`
	Publisher      Publisher       `json:"publisher"`
	RepositoryURL  string          `json:"repository_url"`
	Homepage       string          `json:"homepage,omitempty"`
	IconURL        string          `json:"icon_url,omitempty"`
	License        string          `json:"license"`
	Keywords       []string        `json:"keywords"`
	Recommended    bool            `json:"recommended"`
	Category       string          `json:"category,omitempty"`
	CurrentRelease *CurrentRelease `json:"current_release,omitempty"`
}

type Publisher struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CurrentRelease struct {
	Version        string  `json:"version"`
	PublishedAt    string  `json:"published_at"`
	MinCoreVersion string  `json:"min_core_version"`
	Assets         []Asset `json:"assets"`
}

type Asset struct {
	Platform      string `json:"platform"`
	URL           string `json:"url"`
	ArchiveSHA256 string `json:"archive_sha256"`
}

type Source struct {
	ID       string
	Name     string
	URL      string
	Official bool
}

type CachedCatalog struct {
	SourceID    string
	Payload     []byte
	RefreshedAt time.Time
}

type SourceView struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Official    bool       `json:"official"`
	Cached      bool       `json:"cached"`
	RefreshedAt *time.Time `json:"refreshed_at,omitempty"`
	EntryCount  int        `json:"entry_count"`
}

type ReleaseView struct {
	Version        string    `json:"version"`
	PublishedAt    time.Time `json:"published_at"`
	MinCoreVersion string    `json:"min_core_version"`
	Compatible     bool      `json:"compatible"`
	AssetAvailable bool      `json:"asset_available"`
}

type EntryView struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Summary          string       `json:"summary"`
	Description      string       `json:"description,omitempty"`
	Publisher        Publisher    `json:"publisher"`
	RepositoryURL    string       `json:"repository_url"`
	Homepage         string       `json:"homepage,omitempty"`
	IconURL          string       `json:"icon_url,omitempty"`
	License          string       `json:"license"`
	Keywords         []string     `json:"keywords"`
	Recommended      bool         `json:"recommended"`
	Category         string       `json:"category,omitempty"`
	LatestRelease    *ReleaseView `json:"latest_release,omitempty"`
	InstalledVersion string       `json:"installed_version,omitempty"`
	InstallState     string       `json:"install_state"`
}

type Query struct {
	SourceID string
	Text     string
	Sort     string
	Cursor   int
	Limit    int
}

type ListResult struct {
	Items      []EntryView `json:"items"`
	Total      int         `json:"total"`
	NextCursor string      `json:"next_cursor,omitempty"`
	Source     SourceView  `json:"source"`
}

type DetailResult struct {
	Plugin   EntryView     `json:"plugin"`
	Releases []ReleaseView `json:"releases"`
	Source   SourceView    `json:"source"`
}

type SourceInput struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type InspectionRequest struct {
	SourceID string
	PluginID string
}

type InspectionResult struct {
	Inspection           plugins.InstallInspection `json:"inspection"`
	ConfirmationRequired bool                      `json:"confirmation_required"`
	ConfirmationReasons  []string                  `json:"confirmation_reasons"`
}

type InstallRequest struct {
	PluginID             string
	InspectionID         string
	PackageSHA256        string
	TrustedCodeConfirmed bool
}

type Installer interface {
	plugins.InstallInspector
	plugins.InstallCoordinator
}

type Repository interface {
	ListSources(context.Context) ([]Source, error)
	CreateSource(context.Context, Source) error
	UpdateSource(context.Context, Source) error
	DeleteSource(context.Context, string) error
	LoadCatalogs(context.Context) ([]CachedCatalog, error)
	SaveCatalog(context.Context, CachedCatalog) error
}

type ServiceAPI interface {
	Sources() []SourceView
	CreateSource(context.Context, SourceInput) (SourceView, error)
	UpdateSource(context.Context, string, SourceInput) (SourceView, error)
	DeleteSource(context.Context, string) error
	List(Query) (ListResult, error)
	Get(string, string) (DetailResult, bool)
	Refresh(context.Context, string) (SourceView, error)
	Inspect(context.Context, InspectionRequest) (InspectionResult, error)
	Install(context.Context, InstallRequest) (string, error)
}

func invalidCatalog(format string, args ...any) error {
	return errorWithCode(CodeCatalogUnavailable, fmt.Errorf(format, args...))
}
