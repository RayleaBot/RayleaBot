package releaseupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)

// Release metadata is not a security boundary. Release tooling validates the
// full contract; readers ignore unknown fields and check only the values they
// use, so older installations keep discovering newer releases.
func decodeManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if _, err := parseSemanticVersion(manifest.Version); err != nil {
		return Manifest{}, fmt.Errorf("invalid version: %w", err)
	}
	if err := validateHTTPSURL(manifest.ReleaseNotesRef); err != nil {
		return Manifest{}, fmt.Errorf("invalid release_notes_ref: %w", err)
	}
	return manifest, nil
}

func selectArtifact(manifest Manifest, artifactID string) (Artifact, error) {
	if artifactID == "" {
		return Artifact{}, errors.New("build_info.json does not name an artifact")
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.ArtifactID != artifactID {
			continue
		}
		if !fileNamePattern.MatchString(artifact.FileName) {
			return Artifact{}, errors.New("file_name must be a safe basename")
		}
		if artifact.ArchiveSizeBytes < 1 {
			return Artifact{}, errors.New("archive_size_bytes must be positive")
		}
		if artifact.UpdateMode != "guided" && artifact.UpdateMode != "manual" {
			return Artifact{}, errors.New("invalid update_mode")
		}
		return artifact, nil
	}
	return Artifact{}, fmt.Errorf("release does not contain %s", artifactID)
}

func validateHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("URL must use HTTPS without userinfo")
	}
	return nil
}

func DecodeBuildInfo(data []byte) (BuildInfo, error) {
	var buildInfo BuildInfo
	if err := json.Unmarshal(data, &buildInfo); err != nil {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "decode build_info.json", err)
	}
	if _, err := parseSemanticVersion(buildInfo.Version); err != nil {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "validate build_info.json", fmt.Errorf("invalid version: %w", err))
	}
	return buildInfo, nil
}
