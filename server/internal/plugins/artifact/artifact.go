package artifact

import (
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const (
	Version         = "1"
	ManifestVersion = "2"
	BridgeVersion   = "2"
)

var (
	ErrInvalid          = errors.New("plugin artifact invalid")
	ErrPlatformMismatch = errors.New("plugin platform mismatch")
)

type File struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Document struct {
	ArtifactVersion string `json:"artifact_version"`
	PluginID        string `json:"plugin_id"`
	PluginVersion   string `json:"plugin_version"`
	TargetPlatform  string `json:"target_platform"`
	ManifestSHA256  string `json:"manifest_sha256"`
	Files           []File `json:"files"`
}

type Manifest struct {
	ID                    string        `json:"id"`
	Name                  string        `json:"name"`
	Version               string        `json:"version"`
	ManifestVersion       string        `json:"manifest_version"`
	PluginProtocolVersion string        `json:"plugin_protocol_version"`
	Runtime               string        `json:"runtime"`
	Entry                 string        `json:"entry"`
	Platforms             []string      `json:"platforms"`
	ManagementUI          *ManagementUI `json:"management_ui,omitempty"`
}

type ManagementUI struct {
	Pages []ManagementPage `json:"pages"`
}

type ManagementPage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Entry string `json:"entry"`
}

type Verified struct {
	Root        string
	Document    Document
	Manifest    Manifest
	BackendPath string
	UIAvailable bool
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
	if manifest.ManifestVersion != ManifestVersion || manifest.Runtime != "go" || manifest.PluginProtocolVersion != "1" {
		return Verified{}, invalid("unsupported plugin manifest", nil)
	}
	if document.PluginID != manifest.ID || document.PluginVersion != manifest.Version {
		return Verified{}, invalid("artifact identity does not match info.json", nil)
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	if document.ManifestSHA256 != hex.EncodeToString(manifestDigest[:]) {
		return Verified{}, invalid("info.json digest does not match artifact.json", nil)
	}
	if !contains(manifest.Platforms, document.TargetPlatform) {
		return Verified{}, invalid("target platform is not declared by info.json", nil)
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
	declared := make(map[string]File, len(document.Files))
	roles := make(map[string]string, len(document.Files))
	backendCount := 0
	manifestCount := 0
	for _, item := range document.Files {
		path, err := safeRelativePath(item.Path)
		if err != nil {
			return Verified{}, invalid("invalid artifact file path", err)
		}
		key := strings.ToLower(filepath.ToSlash(path))
		if _, exists := declared[key]; exists {
			return Verified{}, invalid("artifact contains duplicate file paths", nil)
		}
		declared[key] = item
		roles[key] = item.Role
		switch item.Role {
		case "backend":
			backendCount++
		case "manifest":
			manifestCount++
		}
	}
	if backendCount != 1 {
		return Verified{}, invalid("artifact must declare exactly one backend file", nil)
	}
	if manifestCount != 1 || roles["info.json"] != "manifest" {
		return Verified{}, invalid("artifact must declare info.json as its only manifest file", nil)
	}

	actual := make(map[string]string, len(document.Files))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if relative == "artifact.json" {
			return nil
		}
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
	if len(actual) != len(declared) {
		return Verified{}, invalid("artifact file inventory does not match package contents", nil)
	}

	var backendPath string
	uiEntries := make([]string, 0)
	for key, item := range declared {
		relative, exists := actual[key]
		if !exists {
			return Verified{}, invalid("artifact inventory references a missing file: "+item.Path, nil)
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			return Verified{}, invalid("stat artifact file", err)
		}
		if info.Size() != item.Size {
			return Verified{}, invalid("artifact size mismatch: "+item.Path, nil)
		}
		digest, err := fileDigest(path)
		if err != nil {
			return Verified{}, invalid("hash artifact file", err)
		}
		if digest != item.SHA256 {
			return Verified{}, invalid("artifact digest mismatch: "+item.Path, nil)
		}
		if item.Role == "backend" {
			backendPath = path
		}
	}
	for key, relative := range actual {
		if _, exists := declared[key]; !exists {
			return Verified{}, invalid("artifact contains an undeclared file: "+relative, nil)
		}
	}

	expectedBackend := manifest.Entry
	if document.TargetPlatform == "windows-x64" {
		expectedBackend += ".exe"
	}
	if filepath.ToSlash(relativeTo(root, backendPath)) != expectedBackend {
		return Verified{}, invalid("backend file does not match manifest entry", nil)
	}
	if err := validateBinary(backendPath, document.TargetPlatform); err != nil {
		return Verified{}, err
	}
	for _, page := range managementPages(manifest) {
		key := strings.ToLower(filepath.ToSlash(page.Entry))
		if roles[key] != "ui" {
			return Verified{}, invalid("management UI entry is not declared as a UI file: "+page.Entry, nil)
		}
		uiEntries = append(uiEntries, filepath.ToSlash(page.Entry))
	}
	sort.Strings(uiEntries)
	return Verified{BackendPath: backendPath, UIAvailable: len(uiEntries) > 0, UIEntries: uiEntries}, nil
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
		defer file.Close()
		if file.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
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
		defer file.Close()
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
		defer file.Close()
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

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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

func managementPages(manifest Manifest) []ManagementPage {
	if manifest.ManagementUI == nil {
		return nil
	}
	return manifest.ManagementUI.Pages
}

func relativeTo(root, path string) string {
	relative, _ := filepath.Rel(root, path)
	return relative
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func invalid(message string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", ErrInvalid, message)
	}
	return fmt.Errorf("%w: %s: %v", ErrInvalid, message, err)
}
