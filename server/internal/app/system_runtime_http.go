package app

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
}

type managedRuntimePrepareReport struct {
	Kind               string
	ArchivePath        string
	StoreRoot          string
	UsedPreparedStore  bool
	UsedCachedArchive  bool
	AttemptedSources   []string
	SelectedSource     string
	PreparedEntrypoint string
}
