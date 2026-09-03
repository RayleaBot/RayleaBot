package pluginbuild

import (
	"bytes"
	"context"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PackConfig struct {
	PluginDir            string
	Executable           string
	OutputDir            string
	TargetPlatform       string
	MappedAssets         []AssetMapping
	KeepExpandedArtifact bool
}

type Inspection struct {
	PluginID       string
	Version        string
	TargetPlatform string
	Entry          string
	FileCount      int
}

type ProjectInspection struct {
	PluginID        string `json:"plugin_id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestVersion string `json:"manifest_version"`
}

func InspectProject(pluginDir string) (ProjectInspection, error) {
	pluginDir, err := filepath.Abs(pluginDir)
	if err != nil {
		return ProjectInspection{}, fmt.Errorf("pluginbuild: resolve plugin directory: %w", err)
	}
	payload, err := os.ReadFile(filepath.Join(pluginDir, "info.json"))
	if err != nil {
		return ProjectInspection{}, fmt.Errorf("pluginbuild: read plugin manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return ProjectInspection{}, fmt.Errorf("pluginbuild: parse plugin manifest: %w", err)
	}
	if err := validateManifest(manifest, ""); err != nil {
		return ProjectInspection{}, err
	}
	return ProjectInspection{
		PluginID: manifest.ID, Name: manifest.Name, Version: manifest.Version, ManifestVersion: manifest.ManifestVersion,
	}, nil
}

func Pack(_ context.Context, config PackConfig) (Result, error) {
	pluginDir, err := filepath.Abs(config.PluginDir)
	if err != nil {
		return Result{}, fmt.Errorf("pluginbuild: resolve plugin directory: %w", err)
	}
	infoBytes, err := os.ReadFile(filepath.Join(pluginDir, "info.json"))
	if err != nil {
		return Result{}, fmt.Errorf("pluginbuild: read plugin manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(infoBytes, &manifest); err != nil {
		return Result{}, fmt.Errorf("pluginbuild: parse plugin manifest: %w", err)
	}
	if err := validateManifest(manifest, config.TargetPlatform); err != nil {
		return Result{}, err
	}
	target, err := resolveTarget(config.TargetPlatform)
	if err != nil {
		return Result{}, err
	}
	executable, err := filepath.Abs(config.Executable)
	if err != nil {
		return Result{}, fmt.Errorf("pluginbuild: resolve executable: %w", err)
	}
	if err := validateNativeExecutable(executable, config.TargetPlatform); err != nil {
		return Result{}, err
	}
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(pluginDir, "dist")
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return Result{}, fmt.Errorf("pluginbuild: resolve output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("pluginbuild: create output directory: %w", err)
	}
	staging, err := os.MkdirTemp(outputDir, "."+manifest.ID+"-"+config.TargetPlatform+"-")
	if err != nil {
		return Result{}, fmt.Errorf("pluginbuild: create artifact staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	root := filepath.Join(staging, manifest.ID)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), infoBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("pluginbuild: copy plugin manifest: %w", err)
	}
	entry := filepath.ToSlash(filepath.Join("bin", manifest.ID+target.EXE))
	entryPath := filepath.Join(root, filepath.FromSlash(entry))
	mode := fs.FileMode(0o755)
	if target.GOOS == "windows" {
		mode = 0o644
	}
	if err := copyFile(executable, entryPath, mode); err != nil {
		return Result{}, fmt.Errorf("pluginbuild: copy native executable: %w", err)
	}
	if err := copyPackDefaults(pluginDir, root); err != nil {
		return Result{}, err
	}
	if err := copyAssets(pluginDir, root, nil, config.MappedAssets); err != nil {
		return Result{}, err
	}
	if err := copyLicense(pluginDir, root); err != nil {
		return Result{}, err
	}
	return finalizeArtifact(root, staging, outputDir, manifest, entry, config.TargetPlatform, config.KeepExpandedArtifact)
}

func copyPackDefaults(pluginDir, artifactRoot string) error {
	for _, item := range []AssetMapping{
		{Source: "assets", Destination: "assets"},
		{Source: "templates", Destination: "templates"},
		{Source: "THIRD_PARTY_NOTICES.md", Destination: "THIRD_PARTY_NOTICES.md"},
		{Source: "sbom.spdx.json", Destination: "sbom.spdx.json"},
	} {
		if _, err := os.Stat(filepath.Join(pluginDir, filepath.FromSlash(item.Source))); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := copyAsset(pluginDir, artifactRoot, item.Source, item.Destination); err != nil {
			return err
		}
	}
	uiSource := "ui"
	if _, err := os.Stat(filepath.Join(pluginDir, "ui", "index.html")); os.IsNotExist(err) {
		uiSource = "ui/dist"
	}
	if _, err := os.Stat(filepath.Join(pluginDir, filepath.FromSlash(uiSource), "index.html")); err == nil {
		if err := copyAsset(pluginDir, artifactRoot, uiSource, "ui"); err != nil {
			return err
		}
	}
	return nil
}

func Inspect(root, expectedPlatform string) (Inspection, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: resolve artifact root: %w", err)
	}
	artifactBytes, err := os.ReadFile(filepath.Join(root, "artifact.json"))
	if err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: read artifact.json: %w", err)
	}
	var artifact Artifact
	if err := decodeArtifactJSON(artifactBytes, &artifact); err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: parse artifact.json: %w", err)
	}
	if artifact.ArtifactVersion != ArtifactVersion {
		return Inspection{}, fmt.Errorf("pluginbuild: artifact_version must be %s", ArtifactVersion)
	}
	if expectedPlatform != "" && artifact.TargetPlatform != expectedPlatform {
		return Inspection{}, fmt.Errorf("pluginbuild: artifact targets %s, expected %s", artifact.TargetPlatform, expectedPlatform)
	}
	if _, err := resolveTarget(artifact.TargetPlatform); err != nil {
		return Inspection{}, err
	}
	infoBytes, err := os.ReadFile(filepath.Join(root, "info.json"))
	if err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: read info.json: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(infoBytes, &manifest); err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: parse info.json: %w", err)
	}
	if err := validateManifest(manifest, artifact.TargetPlatform); err != nil {
		return Inspection{}, err
	}
	entry, err := canonicalArtifactPath(artifact.Entry)
	if err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: invalid artifact entry: %w", err)
	}
	actual := make(map[string]string)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link is forbidden: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular file is forbidden: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		key := strings.ToLower(relative)
		if previous, exists := actual[key]; exists {
			return fmt.Errorf("case-insensitive file collision: %s and %s", previous, relative)
		}
		actual[key] = relative
		return nil
	})
	if err != nil {
		return Inspection{}, fmt.Errorf("pluginbuild: inventory artifact: %w", err)
	}
	if _, exists := actual["info.json"]; !exists {
		return Inspection{}, errors.New("pluginbuild: info.json is missing")
	}
	if _, exists := actual[strings.ToLower(filepath.ToSlash(entry))]; !exists {
		return Inspection{}, errors.New("pluginbuild: artifact entry is missing")
	}
	if err := validateNativeExecutable(filepath.Join(root, filepath.FromSlash(artifact.Entry)), artifact.TargetPlatform); err != nil {
		return Inspection{}, err
	}
	if manifest.ManagementUI != nil {
		uiEntry, err := canonicalArtifactPath(manifest.ManagementUI.Entry)
		if err != nil {
			return Inspection{}, fmt.Errorf("pluginbuild: invalid management UI entry: %w", err)
		}
		key := strings.ToLower(filepath.ToSlash(uiEntry))
		if _, exists := actual[key]; !exists {
			return Inspection{}, fmt.Errorf("pluginbuild: management UI entry is missing: %s", manifest.ManagementUI.Entry)
		}
	}
	return Inspection{
		PluginID:       manifest.ID,
		Version:        manifest.Version,
		TargetPlatform: artifact.TargetPlatform,
		Entry:          artifact.Entry,
		FileCount:      len(actual),
	}, nil
}

func decodeArtifactJSON(payload []byte, destination *Artifact) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("artifact JSON must contain exactly one value")
		}
		return err
	}
	return nil
}

func canonicalArtifactPath(value string) (string, error) {
	if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", errors.New("artifact path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact path escapes package root")
	}
	if filepath.ToSlash(clean) != value || value == "artifact.json" {
		return "", errors.New("artifact path is not canonical")
	}
	return clean, nil
}

func validateNativeExecutable(path, platform string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("pluginbuild: inspect native executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("pluginbuild: native entry must be a regular file")
	}
	switch platform {
	case "windows-x64":
		file, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("pluginbuild: entry is not a valid PE executable: %w", err)
		}
		defer file.Close()
		if file.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return errors.New("pluginbuild: PE entry architecture must be amd64")
		}
	case "linux-x64":
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			return errors.New("pluginbuild: Unix entry is not executable")
		}
		file, err := elf.Open(path)
		if err != nil {
			return fmt.Errorf("pluginbuild: entry is not a valid ELF executable: %w", err)
		}
		defer file.Close()
		if file.Machine != elf.EM_X86_64 {
			return errors.New("pluginbuild: ELF entry architecture must be x86_64")
		}
	case "macos-arm64":
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			return errors.New("pluginbuild: Unix entry is not executable")
		}
		file, err := macho.Open(path)
		if err != nil {
			return fmt.Errorf("pluginbuild: entry is not a valid Mach-O executable: %w", err)
		}
		defer file.Close()
		if file.Cpu != macho.CpuArm64 {
			return errors.New("pluginbuild: Mach-O entry architecture must be arm64")
		}
	default:
		return fmt.Errorf("pluginbuild: unsupported target platform %q", platform)
	}
	return nil
}
