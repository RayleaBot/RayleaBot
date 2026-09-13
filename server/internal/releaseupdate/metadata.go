package releaseupdate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	gitCommitPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`)
	fileNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`)
)

type artifactMatrixEntry struct {
	platform     string
	smokeProfile string
}

var artifactMatrix = map[string]artifactMatrixEntry{
	ArtifactWindowsX64Full: {platform: "windows-x64", smokeProfile: "windows_full_smoke"},
	ArtifactLinuxX64Full:   {platform: "linux-x64", smokeProfile: "linux_full_smoke"},
	ArtifactMacOSARM64Full: {platform: "macos-arm64", smokeProfile: "macos_full_smoke"},
	ArtifactLinuxX64Server: {platform: "linux-x64", smokeProfile: "linux_server_smoke"},
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.ManifestVersion != 2 {
		return fmt.Errorf("unsupported manifest version")
	}
	if _, err := parseSemanticVersion(manifest.Version); err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}
	if !gitCommitPattern.MatchString(manifest.GitCommit) {
		return fmt.Errorf("invalid git_commit")
	}
	for _, value := range []string{manifest.BuiltAt, manifest.PublishedAt} {
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("invalid release timestamp: %w", err)
		}
	}
	if manifest.Channel != "stable" && manifest.Channel != "beta" {
		return fmt.Errorf("invalid channel")
	}
	if strings.TrimSpace(manifest.ConfigSchemaVersion) == "" || strings.TrimSpace(manifest.DBSchemaVersion) == "" || strings.TrimSpace(manifest.PluginProtocolVersion) == "" {
		return fmt.Errorf("schema and protocol versions are required")
	}
	if manifest.PluginManifestVersion != PluginManifestVersion || manifest.PluginUIBridgeVersion != PluginUIBridgeVersion {
		return fmt.Errorf("release belongs to an unsupported plugin epoch")
	}
	if err := validateHTTPSURL(manifest.ReleaseNotesRef); err != nil {
		return fmt.Errorf("invalid release_notes_ref: %w", err)
	}
	if len(manifest.Artifacts) == 0 || len(manifest.Artifacts) > 16 {
		return fmt.Errorf("artifacts must contain between 1 and 16 entries")
	}
	seenIDs := make(map[string]struct{}, len(manifest.Artifacts))
	seenFiles := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if err := validateArtifact(artifact); err != nil {
			return fmt.Errorf("artifact %q: %w", artifact.ArtifactID, err)
		}
		if _, duplicate := seenIDs[artifact.ArtifactID]; duplicate {
			return fmt.Errorf("duplicate artifact_id %q", artifact.ArtifactID)
		}
		fileKey := strings.ToLower(artifact.FileName)
		if _, duplicate := seenFiles[fileKey]; duplicate {
			return fmt.Errorf("duplicate artifact file_name %q", artifact.FileName)
		}
		seenIDs[artifact.ArtifactID] = struct{}{}
		seenFiles[fileKey] = struct{}{}
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	matrix, supported := artifactMatrix[artifact.ArtifactID]
	if !supported || artifact.Platform != matrix.platform || artifact.SmokeProfile != matrix.smokeProfile {
		return fmt.Errorf("artifact id, platform, and smoke profile do not match the release matrix")
	}
	if !fileNamePattern.MatchString(artifact.FileName) || strings.ContainsAny(artifact.FileName, `/\\:`) {
		return fmt.Errorf("file_name must be a safe basename")
	}
	if artifact.ArchiveSizeBytes < 1 || artifact.ArchiveSizeBytes > 2<<30 || artifact.ExpandedSizeBytes < 1 || artifact.ExpandedSizeBytes > 8<<30 {
		return fmt.Errorf("artifact size is outside the allowed range")
	}
	if artifact.FileCount < 1 || artifact.FileCount > 100_000 {
		return fmt.Errorf("file_count is outside the allowed range")
	}
	if artifact.SupportLevel != "first_class" && artifact.SupportLevel != "experimental" {
		return fmt.Errorf("invalid support_level")
	}
	if artifact.UpdateMode != "guided" && artifact.UpdateMode != "manual" {
		return fmt.Errorf("invalid update_mode")
	}
	return nil
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
	if err := decodeStrictJSON(data, &buildInfo); err != nil {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "decode build_info.json", err)
	}
	if _, err := parseSemanticVersion(buildInfo.Version); err != nil || !gitCommitPattern.MatchString(buildInfo.GitCommit) || buildInfo.ArtifactID == "" || buildInfo.BuiltAt == "" || buildInfo.PluginManifestVersion != PluginManifestVersion || buildInfo.PluginUIBridgeVersion != PluginUIBridgeVersion {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "validate build_info.json", errors.New("invalid build metadata"))
	}
	if _, supported := artifactMatrix[buildInfo.ArtifactID]; !supported {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "validate build_info.json", errors.New("unsupported artifact_id"))
	}
	if _, err := time.Parse(time.RFC3339, buildInfo.BuiltAt); err != nil {
		return BuildInfo{}, errorWithCode(CodeManifestInvalid, "validate build_info.json", errors.New("invalid built_at"))
	}
	return buildInfo, nil
}
