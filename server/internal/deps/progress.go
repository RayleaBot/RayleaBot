package deps

import "strings"

type PrepareProgress struct {
	Kind             string `json:"kind"`
	Label            string `json:"label"`
	ResourceID       string `json:"resource_id,omitempty"`
	Version          string `json:"version,omitempty"`
	SourceLabel      string `json:"source_label,omitempty"`
	SourceURL        string `json:"source_url,omitempty"`
	ArchivePath      string `json:"archive_path,omitempty"`
	StoreRoot        string `json:"store_root,omitempty"`
	Stage            string `json:"stage"`
	Status           string `json:"status"`
	Progress         int    `json:"progress,omitempty"`
	DownloadedBytes  int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes       int64  `json:"total_bytes,omitempty"`
	ExtractedEntries int    `json:"extracted_entries,omitempty"`
	TotalEntries     int    `json:"total_entries,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Error            string `json:"error,omitempty"`
}

type PrepareProgressReporter func(PrepareProgress)

type PrepareOptions struct {
	Progress PrepareProgressReporter
}

type downloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Progress        int
}

type extractProgress struct {
	ExtractedEntries int
	TotalEntries     int
	Progress         int
}

func (p PrepareProgress) withResource(resource *Resource, archivePath, storeRoot string) PrepareProgress {
	if resource == nil {
		p.Label = managedResourceLabel(p.Kind)
		p.ArchivePath = strings.TrimSpace(archivePath)
		p.StoreRoot = strings.TrimSpace(storeRoot)
		return p
	}
	p.Kind = resource.Kind
	p.Label = managedResourceLabel(resource.Kind)
	p.ResourceID = resource.ID
	p.Version = resource.Version
	p.ArchivePath = strings.TrimSpace(archivePath)
	p.StoreRoot = strings.TrimSpace(storeRoot)
	return p
}

func emitPrepareProgress(reporter PrepareProgressReporter, event PrepareProgress) {
	if reporter == nil {
		return
	}
	if event.Label == "" {
		event.Label = managedResourceLabel(event.Kind)
	}
	reporter(event)
}

func prepareProgressPercent(done, total int64) int {
	if total <= 0 || done <= 0 {
		return 0
	}
	percent := int((done * 100) / total)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}
