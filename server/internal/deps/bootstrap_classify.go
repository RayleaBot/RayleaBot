package deps

import (
	"errors"
	"path/filepath"
	"strings"
)

func classifyBootstrapError(repoRoot, kind string, resource *Resource, stage string, selectedSource string, attemptedSources []string, err error) error {
	if err == nil {
		return nil
	}
	archivePath := ""
	storeRoot := ""
	if resource != nil {
		archivePath = filepath.Join(CacheRoot(repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat))
		storeRoot = StoreRoot(repoRoot, resource)
	}
	return &BootstrapError{
		Kind:             kind,
		Stage:            stage,
		SelectedSource:   strings.TrimSpace(selectedSource),
		AttemptedSources: append([]string(nil), attemptedSources...),
		ArchivePath:      archivePath,
		StoreRoot:        storeRoot,
		Remediation:      bootstrapRemediation(kind, archivePath, storeRoot),
		Message:          bootstrapMessage(kind, stage),
		Err:              err,
	}
}

func (m *Manager) classifyBootstrapErrorWithProgress(reporter PrepareProgressReporter, kind string, resource *Resource, stage string, selectedSource string, attemptedSources []string, err error) error {
	bootstrapErr := classifyBootstrapError(m.repoRoot, kind, resource, stage, selectedSource, attemptedSources, err)
	if bootstrapErr == nil {
		return nil
	}
	var details *BootstrapError
	if errors.As(bootstrapErr, &details) {
		sourceURL := strings.TrimSpace(selectedSource)
		if sourceURL == "" && len(attemptedSources) > 0 {
			sourceURL = strings.TrimSpace(attemptedSources[len(attemptedSources)-1])
		}
		emitPrepareProgress(reporter, PrepareProgress{
			Kind:        kind,
			Stage:       stage,
			Status:      "failed",
			SourceURL:   sourceURL,
			ArchivePath: details.ArchivePath,
			StoreRoot:   details.StoreRoot,
			Summary:     details.Message,
			Error:       err.Error(),
		}.withResource(resource, details.ArchivePath, details.StoreRoot))
	}
	return bootstrapErr
}
