package deps

type Manifest struct {
	ManifestVersion int        `json:"manifest_version"`
	Resources       []Resource `json:"resources"`
}
type ResourceSource struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}
type Resource struct {
	ID            string              `json:"id"`
	Kind          string              `json:"kind"`
	Version       string              `json:"version"`
	Platform      string              `json:"platform"`
	Sources       []ResourceSource    `json:"sources"`
	SHA256        string              `json:"sha256"`
	ArchiveFormat string              `json:"archive_format"`
	Entrypoints   map[string][]string `json:"entrypoints"`
}
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
