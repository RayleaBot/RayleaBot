package deps

import depsmanifest "github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"

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
