package deps

import (
	"context"
	"errors"

	depsmanifest "github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"
)

type Manifest = depsmanifest.Manifest
type ResourceSource = depsmanifest.ResourceSource
type Resource = depsmanifest.Resource

type PreparedResource struct {
	Resource    Resource
	Root        string
	Entrypoints map[string]string
}
type BootstrapInspection struct {
	Kind                 string
	Resource             *Resource
	ArchivePath          string
	StoreRoot            string
	MetadataComplete     bool
	CachedArchivePresent bool
	PreparedStorePresent bool
	SystemBrowserPath    string
}
type PrepareReport struct {
	Kind               string
	Resource           Resource
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	AttemptedSources   []string
	SelectedSource     string
	PreparedEntrypoint string
	Entrypoints        map[string]string
	UsedSystemBrowser  bool
}
type BootstrapError struct {
	Kind             string
	Stage            string
	SelectedSource   string
	AttemptedSources []string
	ArchivePath      string
	StoreRoot        string
	Remediation      string
	Message          string
	Err              error
}

// Runtime exposes dependency operations that may prepare or resolve runtime entrypoints.
type Runtime struct {
	manager *Manager
}

func NewRuntime(repoRoot string) *Runtime {
	return &Runtime{manager: NewManager(repoRoot)}
}

func (r *Runtime) ResolveEntrypoint(ctx context.Context, kind, name string) (string, error) {
	if r == nil || r.manager == nil {
		return "", errManagerRequired()
	}
	return r.manager.ResolveEntrypoint(ctx, kind, name)
}

func (r *Runtime) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	if r == nil || r.manager == nil {
		return "", errManagerRequired()
	}
	return r.manager.ResolvePreparedEntrypoint(kind, name)
}

func (r *Runtime) PrepareWithReport(ctx context.Context, kind string) (*PrepareReport, error) {
	if r == nil || r.manager == nil {
		return nil, errManagerRequired()
	}
	return r.manager.PrepareWithReport(ctx, kind)
}

func (r *Runtime) PrepareWithReportOptions(ctx context.Context, kind string, options PrepareOptions) (*PrepareReport, error) {
	if r == nil || r.manager == nil {
		return nil, errManagerRequired()
	}
	return r.manager.PrepareWithReportOptions(ctx, kind, options)
}

// Diagnostics exposes read-only dependency status checks.
type Diagnostics struct {
	manager *Manager
}

func NewDiagnostics(repoRoot string) *Diagnostics {
	return &Diagnostics{manager: NewManager(repoRoot)}
}

func (d *Diagnostics) InspectRuntime(kind string) (*BootstrapInspection, error) {
	if d == nil || d.manager == nil {
		return nil, errManagerRequired()
	}
	return d.manager.Inspect(kind)
}

func errManagerRequired() error {
	return errors.New("deps manager is required")
}
