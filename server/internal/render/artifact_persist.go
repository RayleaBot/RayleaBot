package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) persistArtifact(request Request, cacheKey string, content []byte) (Result, error) {
	artifactID := buildArtifactID(cacheKey)
	filename := artifactID + outputSuffix(request.Output)
	artifactPath := filepath.Join(s.outputRoot, filename)
	if err := os.WriteFile(artifactPath, content, 0o644); err != nil {
		return Result{}, fmt.Errorf("write render artifact %s: %w", artifactPath, err)
	}

	record := artifactRecord{
		ArtifactID: artifactID,
		CacheKey:   cacheKey,
		Template:   request.Template,
		Theme:      request.Theme,
		Output:     request.Output,
		MIME:       outputMIME(request.Output),
		Filename:   filename,
	}
	recordBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode render artifact record %s: %w", artifactID, err)
	}
	if err := os.WriteFile(filepath.Join(s.outputRoot, artifactID+".json"), recordBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("write render artifact record %s: %w", artifactID, err)
	}

	result := Result{
		ArtifactID: artifactID,
		ImagePath:  fileURL(artifactPath),
		MIME:       record.MIME,
		CacheKey:   cacheKey,
		Template:   request.Template,
		Theme:      request.Theme,
		FromCache:  false,
	}

	s.mu.Lock()
	s.artifacts[artifactID] = Artifact{
		ArtifactID: artifactID,
		MIME:       record.MIME,
		Path:       artifactPath,
	}
	s.mu.Unlock()

	return result, nil
}
