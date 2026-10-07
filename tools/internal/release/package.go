package release

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/archiveio"
	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
)

type PackageOptions struct{ ArtifactID, Version, GitCommit, BuiltAt, OutputDir, ServerBin, WebDist, DepsDir, TemplatesDir, LauncherBundle, SystemdFile, ReleaseNotesRef, LicenseFile, ThirdPartyNotices string }
type Sidecar struct {
	ArtifactID        string `json:"artifact_id"`
	ArchivePath       string `json:"archive_path"`
	FileName          string `json:"file_name"`
	Platform          string `json:"platform"`
	SupportLevel      string `json:"support_level"`
	SmokeProfile      string `json:"smoke_profile"`
	ExpandedSizeBytes int64  `json:"expanded_size_bytes"`
	FileCount         int    `json:"file_count"`
	UpdateMode        string `json:"update_mode"`
}
type BuildInfo struct {
	Version               string `json:"version"`
	GitCommit             string `json:"git_commit"`
	ArtifactID            string `json:"artifact_id"`
	BuiltAt               string `json:"built_at"`
	PluginManifestVersion string `json:"plugin_manifest_version"`
	ReleaseNotesRef       string `json:"release_notes_ref,omitempty"`
}

func Now() string { return time.Now().UTC().Format(time.RFC3339) }
func pluginManifestVersion(root string) (string, error) {
	v, e := contractdata.Value(root, "plugin-info.schema.json", "/properties/manifest_version/const")
	return ordered.Str(v), e
}
func Stage(root string, o PackageOptions) (Sidecar, error) {
	matrix, e := Matrix(root)
	if e != nil {
		return Sidecar{}, e
	}
	a, ok := matrix[o.ArtifactID]
	if !ok {
		return Sidecar{}, fmt.Errorf("unsupported artifact_id: %s", o.ArtifactID)
	}
	if a.LauncherRequired && o.LauncherBundle == "" {
		return Sidecar{}, fmt.Errorf("%s requires --launcher-bundle", o.ArtifactID)
	}
	if o.ArtifactID == "linux-x64-server" && o.SystemdFile == "" {
		return Sidecar{}, errors.New("linux-x64-server requires --systemd-file")
	}
	if o.ArtifactID == "windows-x64-full" {
		if e = checkWindowsLauncher(o.LauncherBundle); e != nil {
			return Sidecar{}, e
		}
	}
	for _, f := range [][2]string{{o.LicenseFile, "LICENSE"}, {o.ThirdPartyNotices, "THIRD_PARTY_NOTICES.md"}} {
		i, e := os.Stat(f[0])
		if e != nil || !i.Mode().IsRegular() || i.Size() == 0 {
			return Sidecar{}, fmt.Errorf("release package requires non-empty %s", f[1])
		}
	}
	name := "RayleaBot-v" + o.Version + "-" + o.ArtifactID
	stage := filepath.Join(o.OutputDir, "staging", name)
	if e = os.RemoveAll(stage); e != nil {
		return Sidecar{}, e
	}
	if e = os.MkdirAll(stage, 0755); e != nil {
		return Sidecar{}, e
	}
	if e = copyFile(o.ServerBin, filepath.Join(stage, filepath.Base(o.ServerBin))); e != nil {
		return Sidecar{}, e
	}
	if a.LauncherRequired {
		if e = copyLauncher(o.LauncherBundle, stage); e != nil {
			return Sidecar{}, e
		}
	}
	if o.ArtifactID == "linux-x64-server" {
		if e = copyFile(o.SystemdFile, filepath.Join(stage, "systemd/rayleabot.service")); e != nil {
			return Sidecar{}, e
		}
	}
	for _, v := range [][2]string{{o.WebDist, "web/dist"}, {o.TemplatesDir, "templates"}} {
		if e = copyTree(v[0], filepath.Join(stage, v[1]), true); e != nil {
			return Sidecar{}, e
		}
	}
	copies := [][2]string{{filepath.Join(o.DepsDir, "manifest.json"), ".deps/manifest.json"}, {o.LicenseFile, "LICENSE"}, {o.ThirdPartyNotices, "THIRD_PARTY_NOTICES.md"}}
	if strings.HasPrefix(a.Platform, "linux-") {
		copies = append(copies, [2]string{filepath.Join(root, "docs/release/linux-desktop-runtime.md"), "LINUX-RUNTIME.md"})
	}
	for _, v := range copies {
		if e = copyFile(v[0], filepath.Join(stage, v[1])); e != nil {
			return Sidecar{}, e
		}
	}
	manifestVersion, e := pluginManifestVersion(root)
	if e != nil {
		return Sidecar{}, e
	}
	if o.BuiltAt == "" {
		o.BuiltAt = Now()
	}
	if e = writeJSON(filepath.Join(stage, "build_info.json"), BuildInfo{o.Version, o.GitCommit, o.ArtifactID, o.BuiltAt, manifestVersion, o.ReleaseNotesRef}); e != nil {
		return Sidecar{}, e
	}
	if e = assertClean(stage); e != nil {
		return Sidecar{}, e
	}
	archive := filepath.Join(o.OutputDir, name+a.Extension)
	absolute, e := filepath.Abs(archive)
	if e != nil {
		return Sidecar{}, e
	}
	sidecar := Sidecar{ArtifactID: o.ArtifactID, ArchivePath: absolute, FileName: filepath.Base(archive), Platform: a.Platform, SupportLevel: a.SupportLevel, SmokeProfile: a.SmokeProfile, UpdateMode: "guided"}
	e = filepath.WalkDir(stage, func(p string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := os.Stat(p)
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			sidecar.ExpandedSizeBytes += info.Size()
			sidecar.FileCount++
		}
		return nil
	})
	if e != nil {
		return Sidecar{}, e
	}
	if e = archiveio.Create(stage, archive, a.Extension == ".zip"); e != nil {
		return Sidecar{}, e
	}
	if e = writeJSON(archive+".artifact.json", sidecar); e != nil {
		return Sidecar{}, e
	}
	return sidecar, nil
}
func LoadSidecar(path string) (Sidecar, error) {
	var s Sidecar
	if e := readJSON(path, &s); e != nil {
		return s, e
	}
	if filepath.Base(s.FileName) != s.FileName {
		return s, errors.New("artifact sidecar file_name must be a basename")
	}
	s.ArchivePath = filepath.Join(filepath.Dir(path), s.FileName)
	i, e := os.Stat(s.ArchivePath)
	if e != nil || !i.Mode().IsRegular() {
		return s, fmt.Errorf("artifact archive is not adjacent to its sidecar: %s", s.ArchivePath)
	}
	return s, nil
}

type MetadataOptions struct{ Version, GitCommit, BuiltAt, ConfigSchemaVersion, DBSchemaVersion, PluginProtocolVersion, ReleaseNotesRef, DownloadBaseURL, OutputDir, Channel, PublishedAt string }
type ManifestArtifact struct {
	ArtifactID        string `json:"artifact_id"`
	FileName          string `json:"file_name"`
	DownloadURL       string `json:"download_url"`
	Platform          string `json:"platform"`
	ArchiveSizeBytes  int64  `json:"archive_size_bytes"`
	ExpandedSizeBytes int64  `json:"expanded_size_bytes"`
	FileCount         int    `json:"file_count"`
	UpdateMode        string `json:"update_mode"`
	SupportLevel      string `json:"support_level"`
	SmokeProfile      string `json:"smoke_profile"`
}
type Manifest struct {
	ManifestVersion       int                `json:"manifest_version"`
	Version               string             `json:"version"`
	GitCommit             string             `json:"git_commit"`
	BuiltAt               string             `json:"built_at"`
	Channel               string             `json:"channel"`
	PublishedAt           string             `json:"published_at"`
	ConfigSchemaVersion   string             `json:"config_schema_version"`
	DBSchemaVersion       string             `json:"db_schema_version"`
	PluginProtocolVersion string             `json:"plugin_protocol_version"`
	PluginManifestVersion string             `json:"plugin_manifest_version"`
	Artifacts             []ManifestArtifact `json:"artifacts"`
	ReleaseNotesRef       string             `json:"release_notes_ref"`
}

func BuildMetadata(root string, o MetadataOptions, sidecars []Sidecar) (string, error) {
	channel, e := Channel(root, o.Version, o.Channel)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(o.OutputDir, 0755); e != nil {
		return "", e
	}
	if o.BuiltAt == "" {
		o.BuiltAt = Now()
	}
	publication := o.PublishedAt
	if publication == "" {
		publication = o.BuiltAt
	}
	published, e := ParseReleaseTime(publication)
	if e != nil {
		return "", e
	}
	version, e := pluginManifestVersion(root)
	if e != nil {
		return "", e
	}
	manifest := Manifest{2, o.Version, o.GitCommit, o.BuiltAt, channel, published.Format(time.RFC3339), o.ConfigSchemaVersion, o.DBSchemaVersion, o.PluginProtocolVersion, version, []ManifestArtifact{}, o.ReleaseNotesRef}
	sidecars = slices.Clone(sidecars)
	slices.SortFunc(sidecars, func(a, b Sidecar) int { return strings.Compare(a.ArtifactID, b.ArtifactID) })
	for _, s := range sidecars {
		i, e := os.Stat(s.ArchivePath)
		if e != nil || !i.Mode().IsRegular() || filepath.Base(s.ArchivePath) != s.FileName || filepath.Base(s.FileName) != s.FileName {
			return "", fmt.Errorf("invalid release artifact path for %s", s.ArtifactID)
		}
		if s.FileCount < 1 || s.FileCount > archiveio.MaxEntries {
			return "", fmt.Errorf("invalid release file count for %s", s.ArtifactID)
		}
		if s.ExpandedSizeBytes < 1 || s.ExpandedSizeBytes > archiveio.MaxExpandedBytes {
			return "", fmt.Errorf("invalid expanded size for %s", s.ArtifactID)
		}
		if i.Size() < 1 || i.Size() > archiveio.MaxArchiveBytes {
			return "", fmt.Errorf("invalid archive size for %s", s.ArtifactID)
		}
		if s.UpdateMode != "guided" && s.UpdateMode != "manual" {
			return "", fmt.Errorf("invalid update mode for %s", s.ArtifactID)
		}
		manifest.Artifacts = append(manifest.Artifacts, ManifestArtifact{s.ArtifactID, s.FileName, strings.TrimRight(o.DownloadBaseURL, "/") + "/" + s.FileName, s.Platform, i.Size(), s.ExpandedSizeBytes, s.FileCount, s.UpdateMode, s.SupportLevel, s.SmokeProfile})
	}
	if e = contractdata.Validate(root, "release-manifest.schema.json", manifest); e != nil {
		return "", e
	}
	p := filepath.Join(o.OutputDir, "release_manifest.v2.json")
	return p, writeJSON(p, manifest)
}
