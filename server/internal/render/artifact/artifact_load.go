package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func Load(outputRoot string) (map[string]Result, map[string]Artifact, error) {
	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("read render output root %s: %w", outputRoot, err)
	}

	cache := make(map[string]Result)
	artifacts := make(map[string]Artifact)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		recordPath := filepath.Join(outputRoot, entry.Name())
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read render artifact record %s: %w", recordPath, err)
		}

		var record artifactRecord
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return nil, nil, fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
		}

		artifactPath := filepath.Join(outputRoot, filepath.Base(record.Filename))
		if !pathWithinRoot(outputRoot, artifactPath) {
			continue
		}
		if _, err := os.Stat(artifactPath); err != nil {
			continue
		}

		result := Result{
			ArtifactID: record.ArtifactID,
			ImagePath:  fileURL(artifactPath),
			MIME:       record.MIME,
			CacheKey:   record.CacheKey,
			Template:   record.Template,
			Theme:      record.Theme,
			FromCache:  true,
		}
		cache[record.CacheKey] = result
		artifacts[record.ArtifactID] = Artifact{
			ArtifactID: record.ArtifactID,
			MIME:       record.MIME,
			Path:       artifactPath,
		}
	}

	return cache, artifacts, nil
}
