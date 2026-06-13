package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) loadArtifacts() error {
	entries, err := os.ReadDir(s.outputRoot)
	if err != nil {
		return fmt.Errorf("read render output root %s: %w", s.outputRoot, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		recordPath := filepath.Join(s.outputRoot, entry.Name())
		recordBytes, err := os.ReadFile(recordPath)
		if err != nil {
			return fmt.Errorf("read render artifact record %s: %w", recordPath, err)
		}

		var record artifactRecord
		if err := json.Unmarshal(recordBytes, &record); err != nil {
			return fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
		}

		artifactPath := filepath.Join(s.outputRoot, filepath.Base(record.Filename))
		if !pathWithinRoot(s.outputRoot, artifactPath) {
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
		s.cache[record.CacheKey] = result
		s.artifacts[record.ArtifactID] = Artifact{
			ArtifactID: record.ArtifactID,
			MIME:       record.MIME,
			Path:       artifactPath,
		}
	}

	return nil
}
