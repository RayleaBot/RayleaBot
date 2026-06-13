package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) LookupArtifact(artifactID string) (Artifact, error) {
	if s == nil {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render service is not available"}
	}
	if !artifactIDPattern.MatchString(strings.TrimSpace(artifactID)) {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found"}
	}

	s.mu.RLock()
	if artifact, ok := s.artifacts[artifactID]; ok {
		s.mu.RUnlock()
		return artifact, nil
	}
	s.mu.RUnlock()

	recordPath := filepath.Join(s.outputRoot, artifactID+".json")
	recordBytes, err := os.ReadFile(recordPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found", Err: err}
		}
		return Artifact{}, fmt.Errorf("read render artifact record %s: %w", recordPath, err)
	}

	var record artifactRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		return Artifact{}, fmt.Errorf("decode render artifact record %s: %w", recordPath, err)
	}

	artifactPath := filepath.Join(s.outputRoot, filepath.Base(record.Filename))
	if !pathWithinRoot(s.outputRoot, artifactPath) {
		return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact path is invalid"}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		if os.IsNotExist(err) {
			return Artifact{}, &Error{Code: "platform.resource_missing", Message: "render artifact was not found", Err: err}
		}
		return Artifact{}, fmt.Errorf("inspect render artifact %s: %w", artifactPath, err)
	}

	artifact := Artifact{
		ArtifactID: record.ArtifactID,
		MIME:       record.MIME,
		Path:       artifactPath,
	}

	s.mu.Lock()
	s.artifacts[artifactID] = artifact
	s.mu.Unlock()

	return artifact, nil
}
