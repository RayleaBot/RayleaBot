package artifact

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"

	"github.com/RayleaBot/RayleaBot/server/internal/contractversions"
)

const (
	Version         = contractversions.PluginArtifactVersion
	ManifestVersion = contractversions.PluginManifestVersion
)

var (
	ErrInvalid             = errors.New("plugin artifact invalid")
	ErrPlatformMismatch    = errors.New("plugin platform mismatch")
	ErrContractUnsupported = errors.New("plugin contract unsupported")
)

type Document struct {
	ArtifactVersion string `json:"artifact_version"`
	TargetPlatform  string `json:"target_platform"`
	Entry           string `json:"entry"`
}

type Manifest struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Version         string        `json:"version"`
	ManifestVersion string        `json:"manifest_version"`
	ManagementUI    *ManagementUI `json:"management_ui,omitempty"`
}

type ManagementUI struct {
	Entry string           `json:"entry"`
	Pages []ManagementPage `json:"pages"`
}

type ManagementPage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Verified struct {
	Root        string
	Document    Document
	Manifest    Manifest
	BackendPath string
	BackendSize int64
	FileCount   int
	UIAvailable bool
	UIFileCount int
	UIEntries   []string
}

type Options struct {
	ExpectedPlatform string
}

func CurrentPlatform() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "windows-x64", nil
	case "linux/amd64":
		return "linux-x64", nil
	case "darwin/arm64":
		return "macos-arm64", nil
	default:
		return "", fmt.Errorf("%w: unsupported host %s/%s", ErrPlatformMismatch, runtime.GOOS, runtime.GOARCH)
	}
}

func Verify(root string, options Options) (Verified, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Verified{}, invalid("resolve artifact root", err)
	}
	rootInfo, err := os.Stat(absoluteRoot)
	if err != nil || !rootInfo.IsDir() {
		return Verified{}, invalid("artifact root must be a directory", err)
	}

	documentBytes, documentValue, err := readJSON(filepath.Join(absoluteRoot, "artifact.json"))
	if err != nil {
		return Verified{}, invalid("read artifact.json", err)
	}
	if version := objectString(documentValue, "artifact_version"); version != Version {
		return Verified{}, fmt.Errorf("%w: artifact_version %q, supported %q", ErrContractUnsupported, version, Version)
	}
	artifactValidator, err := config.CompileJSON(config.PluginArtifactSchemaID, config.PluginArtifactSchemaJSON)
	if err != nil {
		return Verified{}, invalid("compile artifact schema", err)
	}
	if err := artifactValidator.Validate(documentValue); err != nil {
		return Verified{}, invalid("validate artifact.json", err)
	}
	var document Document
	if err := json.Unmarshal(documentBytes, &document); err != nil {
		return Verified{}, invalid("decode artifact.json", err)
	}
	if options.ExpectedPlatform != "" && document.TargetPlatform != options.ExpectedPlatform {
		return Verified{}, fmt.Errorf("%w: package targets %s, host requires %s", ErrPlatformMismatch, document.TargetPlatform, options.ExpectedPlatform)
	}

	manifestBytes, manifestValue, err := readJSON(filepath.Join(absoluteRoot, "info.json"))
	if err != nil {
		return Verified{}, invalid("read info.json", err)
	}
	if version := objectString(manifestValue, "manifest_version"); version != ManifestVersion {
		return Verified{}, fmt.Errorf("%w: manifest_version %q, supported %q", ErrContractUnsupported, version, ManifestVersion)
	}
	manifestValidator, err := config.CompileJSON(config.PluginInfoSchemaID, config.PluginInfoSchemaJSON)
	if err != nil {
		return Verified{}, invalid("compile plugin manifest schema", err)
	}
	if err := manifestValidator.Validate(manifestValue); err != nil {
		return Verified{}, invalid("validate info.json", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Verified{}, invalid("decode info.json", err)
	}
	if manifest.ManifestVersion != ManifestVersion {
		return Verified{}, invalid("unsupported plugin manifest", nil)
	}
	verified, err := verifyFiles(absoluteRoot, document, manifest)
	if err != nil {
		return Verified{}, err
	}
	verified.Document = document
	verified.Manifest = manifest
	verified.Root = absoluteRoot
	return verified, nil
}

func verifyFiles(root string, document Document, manifest Manifest) (Verified, error) {
	entryPath, err := safeRelativePath(document.Entry)
	if err != nil {
		return Verified{}, invalid("invalid artifact entry path", err)
	}
	entryKey := strings.ToLower(filepath.ToSlash(entryPath))

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
		return Verified{}, invalid("inventory artifact files", err)
	}
	if _, exists := actual["info.json"]; !exists {
		return Verified{}, invalid("artifact must contain info.json", nil)
	}
	if _, exists := actual[entryKey]; !exists {
		return Verified{}, invalid("artifact entry is missing", nil)
	}

	backendPath := filepath.Join(root, filepath.FromSlash(document.Entry))
	if err := validateBinary(backendPath, document.TargetPlatform); err != nil {
		return Verified{}, err
	}
	backendInfo, err := os.Stat(backendPath)
	if err != nil {
		return Verified{}, invalid("stat backend binary", err)
	}
	uiEntries := make([]string, 0, 1)
	uiFileCount := 0
	for key := range actual {
		if strings.HasPrefix(key, "ui/") {
			uiFileCount++
		}
	}
	if manifest.ManagementUI != nil {
		uiPath, err := safeRelativePath(manifest.ManagementUI.Entry)
		if err != nil {
			return Verified{}, invalid("invalid management UI entry", err)
		}
		key := strings.ToLower(filepath.ToSlash(uiPath))
		if _, exists := actual[key]; !exists {
			return Verified{}, invalid("management UI entry is missing: "+manifest.ManagementUI.Entry, nil)
		}
		uiEntries = append(uiEntries, filepath.ToSlash(manifest.ManagementUI.Entry))
	}
	return Verified{
		BackendPath: backendPath,
		BackendSize: backendInfo.Size(),
		FileCount:   len(actual),
		UIAvailable: len(uiEntries) > 0,
		UIFileCount: uiFileCount,
		UIEntries:   uiEntries,
	}, nil
}

func validateBinary(path, platform string) error {
	info, err := os.Stat(path)
	if err != nil {
		return invalid("stat backend binary", err)
	}
	if !info.Mode().IsRegular() {
		return invalid("backend must be a regular file", nil)
	}
	switch platform {
	case "windows-x64":
		file, err := pe.Open(path)
		if err != nil {
			return invalid("backend is not a valid PE executable", err)
		}
		defer func(release func() error) { _ = release() }(file.Close)
		if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return invalid("backend PE architecture must be amd64", nil)
		}
	case "linux-x64":
		if info.Mode().Perm()&0o111 == 0 {
			return invalid("Unix backend is not executable", nil)
		}
		file, err := elf.Open(path)
		if err != nil {
			return invalid("backend is not a valid ELF executable", err)
		}
		defer func(release func() error) { _ = release() }(file.Close)
		if file.Machine != elf.EM_X86_64 {
			return invalid("backend ELF architecture must be x86_64", nil)
		}
	case "macos-arm64":
		if info.Mode().Perm()&0o111 == 0 {
			return invalid("Unix backend is not executable", nil)
		}
		file, err := macho.Open(path)
		if err != nil {
			return invalid("backend is not a valid Mach-O executable", err)
		}
		defer func(release func() error) { _ = release() }(file.Close)
		if file.Cpu != macho.CpuArm64 {
			return invalid("backend Mach-O architecture must be arm64", nil)
		}
	default:
		return fmt.Errorf("%w: unsupported package target %s", ErrPlatformMismatch, platform)
	}
	return nil
}

func readJSON(path string) ([]byte, any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var value any
	if err := json.Unmarshal(content, &value); err != nil {
		return nil, nil, err
	}
	return content, value, nil
}

func objectString(value any, key string) string {
	object, _ := value.(map[string]any)
	text, _ := object[key].(string)
	return strings.TrimSpace(text)
}

func safeRelativePath(value string) (string, error) {
	if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", errors.New("path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes artifact root")
	}
	if filepath.ToSlash(clean) != value {
		return "", errors.New("path is not canonical")
	}
	return clean, nil
}

func invalid(message string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", ErrInvalid, message)
	}
	return fmt.Errorf("%w: %s: %v", ErrInvalid, message, err)
}
