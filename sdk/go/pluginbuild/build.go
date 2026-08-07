package pluginbuild

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	ArtifactVersion           = "1"
	ManifestVersion           = "2"
	pluginBuildNodeEnv        = "RAYLEA_PLUGIN_BUILD_NODE"
	pluginBuildCorepackCLIEnv = "RAYLEA_PLUGIN_BUILD_COREPACK_CLI"
)

type Config struct {
	PluginDir            string
	OutputDir            string
	TargetPlatform       string
	BackendPackage       string
	Assets               []string
	MappedAssets         []AssetMapping
	GoCommand            string
	PNPMCommand          string
	SkipUIInstall        bool
	KeepExpandedArtifact bool
}

type AssetMapping struct {
	Source      string
	Destination string
}

type Result struct {
	PluginID       string
	Version        string
	TargetPlatform string
	ArtifactDir    string
	ArchivePath    string
	ArchiveSHA256  string
}

type Manifest struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Version               string   `json:"version"`
	ManifestVersion       string   `json:"manifest_version"`
	PluginProtocolVersion string   `json:"plugin_protocol_version"`
	Runtime               string   `json:"runtime"`
	Entry                 string   `json:"entry"`
	Platforms             []string `json:"platforms"`
	ManagementUI          *struct {
		Pages []struct {
			Entry string `json:"entry"`
		} `json:"pages"`
	} `json:"management_ui,omitempty"`
}

type Artifact struct {
	ArtifactVersion string         `json:"artifact_version"`
	PluginID        string         `json:"plugin_id"`
	PluginVersion   string         `json:"plugin_version"`
	TargetPlatform  string         `json:"target_platform"`
	ManifestSHA256  string         `json:"manifest_sha256"`
	Files           []ArtifactFile `json:"files"`
}

type ArtifactFile struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type target struct {
	GOOS   string
	GOARCH string
	EXE    string
}

func CurrentPlatform() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "windows-x64"
	case "linux/amd64":
		return "linux-x64"
	case "darwin/arm64":
		return "macos-arm64"
	default:
		return ""
	}
}

func Build(ctx context.Context, config Config) (Result, error) {
	pluginDir, err := filepath.Abs(config.PluginDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve plugin directory: %w", err)
	}
	infoPath := filepath.Join(pluginDir, "info.json")
	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return Result{}, fmt.Errorf("read plugin manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(infoBytes, &manifest); err != nil {
		return Result{}, fmt.Errorf("parse plugin manifest: %w", err)
	}
	if err := validateManifest(manifest, config.TargetPlatform); err != nil {
		return Result{}, err
	}
	backendPackage, err := resolveBackendPackage(pluginDir, config.BackendPackage)
	if err != nil {
		return Result{}, err
	}
	config.BackendPackage = backendPackage
	target, err := resolveTarget(config.TargetPlatform)
	if err != nil {
		return Result{}, err
	}
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(pluginDir, "dist")
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve output directory: %w", err)
	}
	if pathWithin(pluginDir, outputDir) && outputDir == pluginDir {
		return Result{}, errors.New("pluginbuild: output directory cannot be the plugin source root")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}
	staging, err := os.MkdirTemp(outputDir, "."+manifest.ID+"-"+config.TargetPlatform+"-")
	if err != nil {
		return Result{}, fmt.Errorf("create artifact staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	root := filepath.Join(staging, manifest.ID)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), infoBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("copy plugin manifest: %w", err)
	}

	binaryPath := filepath.Join(root, filepath.FromSlash(manifest.Entry)+target.EXE)
	if err := runGoBuild(ctx, config, pluginDir, backendPackage, target, binaryPath); err != nil {
		return Result{}, err
	}
	if target.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0o755); err != nil {
			return Result{}, fmt.Errorf("mark backend executable: %w", err)
		}
	}
	if err := buildUI(ctx, config, pluginDir, root); err != nil {
		return Result{}, err
	}
	if err := copyAssets(pluginDir, root, config.Assets, config.MappedAssets); err != nil {
		return Result{}, err
	}
	if err := copyLicense(pluginDir, root); err != nil {
		return Result{}, err
	}
	if err := writeNotices(ctx, config, pluginDir, root); err != nil {
		return Result{}, err
	}
	if err := writeSBOM(ctx, config, pluginDir, root, manifest); err != nil {
		return Result{}, err
	}
	artifact, err := inventory(root, manifest, config.TargetPlatform, infoBytes)
	if err != nil {
		return Result{}, err
	}
	artifactBytes, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode artifact manifest: %w", err)
	}
	artifactBytes = append(artifactBytes, '\n')
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), artifactBytes, 0o644); err != nil {
		return Result{}, fmt.Errorf("write artifact manifest: %w", err)
	}
	archivePath := filepath.Join(outputDir, fmt.Sprintf("%s-%s-%s.zip", manifest.ID, manifest.Version, config.TargetPlatform))
	if err := writeDeterministicZIP(archivePath, staging, manifest.ID); err != nil {
		return Result{}, err
	}
	archiveDigest, err := fileSHA256(archivePath)
	if err != nil {
		return Result{}, err
	}
	artifactDir := ""
	if config.KeepExpandedArtifact {
		artifactDir = filepath.Join(outputDir, config.TargetPlatform, manifest.ID)
		if err := replaceTree(root, artifactDir); err != nil {
			return Result{}, err
		}
	}
	return Result{
		PluginID:       manifest.ID,
		Version:        manifest.Version,
		TargetPlatform: config.TargetPlatform,
		ArtifactDir:    artifactDir,
		ArchivePath:    archivePath,
		ArchiveSHA256:  archiveDigest,
	}, nil
}

func validateManifest(manifest Manifest, platform string) error {
	if manifest.ID == "" || manifest.Version == "" {
		return errors.New("pluginbuild: manifest id and version are required")
	}
	if manifest.ManifestVersion != ManifestVersion {
		return fmt.Errorf("pluginbuild: manifest_version must be %s", ManifestVersion)
	}
	if manifest.PluginProtocolVersion != "1" {
		return errors.New("pluginbuild: plugin_protocol_version must be 1")
	}
	if manifest.Runtime != "go" {
		return errors.New("pluginbuild: runtime must be go")
	}
	if manifest.Entry == "" || filepath.Ext(manifest.Entry) != "" || !strings.HasPrefix(filepath.ToSlash(manifest.Entry), "bin/") {
		return errors.New("pluginbuild: entry must be an extensionless path below bin/")
	}
	for _, supported := range manifest.Platforms {
		if supported == platform {
			return nil
		}
	}
	return fmt.Errorf("pluginbuild: target platform %s is not declared by the manifest", platform)
}

func resolveTarget(platform string) (target, error) {
	switch platform {
	case "windows-x64":
		return target{GOOS: "windows", GOARCH: "amd64", EXE: ".exe"}, nil
	case "linux-x64":
		return target{GOOS: "linux", GOARCH: "amd64"}, nil
	case "macos-arm64":
		return target{GOOS: "darwin", GOARCH: "arm64"}, nil
	default:
		return target{}, fmt.Errorf("pluginbuild: unsupported target platform %q", platform)
	}
}

func resolveBackendPackage(pluginDir, configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" || configured == "." || configured == "./" {
		return ".", nil
	}
	packagePath := filepath.Clean(filepath.FromSlash(configured))
	if filepath.IsAbs(packagePath) || filepath.VolumeName(packagePath) != "" {
		return "", errors.New("pluginbuild: backend package must be relative to the plugin root")
	}
	packageDir := filepath.Join(pluginDir, packagePath)
	if !pathWithin(pluginDir, packageDir) {
		return "", errors.New("pluginbuild: backend package must stay within the plugin root")
	}
	info, err := os.Stat(packageDir)
	if err != nil {
		return "", fmt.Errorf("pluginbuild: inspect backend package: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("pluginbuild: backend package must reference a directory")
	}
	return "./" + filepath.ToSlash(packagePath), nil
}

func runGoBuild(ctx context.Context, config Config, pluginDir, backendPackage string, target target, output string) error {
	command := config.GoCommand
	if command == "" {
		command = "go"
	}
	cmd := exec.CommandContext(ctx, command, "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", output, backendPackage)
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.GOOS, "GOARCH="+target.GOARCH)
	if os.Getenv("RAYLEA_PLUGIN_BUILD_USE_WORKSPACE") != "1" {
		cmd.Env = append(cmd.Env, "GOWORK=off")
	}
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pluginbuild: go build failed: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	return nil
}

func buildUI(ctx context.Context, config Config, pluginDir, artifactRoot string) error {
	uiDir := filepath.Join(pluginDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("pluginbuild: inspect UI package: %w", err)
	}
	command, prefixArgs, err := resolvePNPMCommand(config)
	if err != nil {
		return err
	}
	if !config.SkipUIInstall {
		if _, err := os.Stat(filepath.Join(uiDir, "node_modules")); os.IsNotExist(err) {
			if err := runCommand(ctx, uiDir, command, prefixedArgs(prefixArgs, "install", "--frozen-lockfile")...); err != nil {
				return fmt.Errorf("pluginbuild: install UI dependencies: %w", err)
			}
		}
	}
	if err := runCommand(ctx, uiDir, command, prefixedArgs(prefixArgs, "build")...); err != nil {
		return fmt.Errorf("pluginbuild: build UI: %w", err)
	}
	source := filepath.Join(uiDir, "dist")
	if _, err := os.Stat(filepath.Join(source, "index.html")); err != nil {
		return fmt.Errorf("pluginbuild: UI build did not produce dist/index.html: %w", err)
	}
	return copyTree(source, filepath.Join(artifactRoot, "ui"))
}

func resolvePNPMCommand(config Config) (string, []string, error) {
	if config.PNPMCommand != "" {
		return config.PNPMCommand, nil, nil
	}
	nodeCommand := strings.TrimSpace(os.Getenv(pluginBuildNodeEnv))
	corepackCLI := strings.TrimSpace(os.Getenv(pluginBuildCorepackCLIEnv))
	if nodeCommand != "" || corepackCLI != "" {
		if nodeCommand == "" || corepackCLI == "" {
			return "", nil, fmt.Errorf("pluginbuild: %s and %s must be set together", pluginBuildNodeEnv, pluginBuildCorepackCLIEnv)
		}
		if err := requireRegularFile(nodeCommand); err != nil {
			return "", nil, fmt.Errorf("pluginbuild: inspect managed Node.js executable: %w", err)
		}
		if err := requireRegularFile(corepackCLI); err != nil {
			return "", nil, fmt.Errorf("pluginbuild: inspect managed Corepack CLI: %w", err)
		}
		return nodeCommand, []string{corepackCLI, "pnpm"}, nil
	}
	if runtime.GOOS != "windows" {
		return "pnpm", nil, nil
	}
	nodeCommand, err := findRegularFileOnPath("node.exe", os.Environ())
	if err != nil {
		return "", nil, fmt.Errorf("pluginbuild: locate Node.js for UI build: %w", err)
	}
	corepackCLI = filepath.Join(filepath.Dir(nodeCommand), "node_modules", "corepack", "dist", "corepack.js")
	if err := requireRegularFile(corepackCLI); err != nil {
		return "", nil, fmt.Errorf("pluginbuild: locate Corepack CLI next to Node.js: %w", err)
	}
	return nodeCommand, []string{corepackCLI, "pnpm"}, nil
}

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

func findRegularFileOnPath(name string, environment []string) (string, error) {
	pathValue := ""
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		if found && ((runtime.GOOS == "windows" && strings.EqualFold(key, "PATH")) || key == "PATH") {
			pathValue = value
			break
		}
	}
	directories := filepath.SplitList(pathValue)
	if runtime.GOOS == "windows" {
		directories = strings.Split(pathValue, ";")
	}
	for _, directory := range directories {
		directory = strings.Trim(directory, "\"")
		if directory == "" {
			continue
		}
		candidate := filepath.Join(directory, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			absolute, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return absolute, nil
		}
	}
	return "", fmt.Errorf("%s was not found on PATH", name)
}

func prefixedArgs(prefix []string, args ...string) []string {
	result := make([]string, 0, len(prefix)+len(args))
	result = append(result, prefix...)
	return append(result, args...)
}

func runCommand(ctx context.Context, dir, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	environment := environmentWithValue(os.Environ(), "CI", "true")
	if filepath.IsAbs(command) {
		environment = environmentWithPathPrefix(environment, filepath.Dir(command))
	}
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", command, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func environmentWithValue(environment []string, key, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		entryKey, _, found := strings.Cut(entry, "=")
		matches := entryKey == key
		if runtime.GOOS == "windows" {
			matches = strings.EqualFold(entryKey, key)
		}
		if found && !matches {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}

func environmentWithPathPrefix(environment []string, directory string) []string {
	result := make([]string, 0, len(environment)+1)
	currentPath := ""
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		isPath := key == "PATH"
		if runtime.GOOS == "windows" {
			isPath = strings.EqualFold(key, "PATH")
		}
		if isPath {
			if currentPath == "" {
				currentPath = value
			}
			continue
		}
		if found {
			result = append(result, entry)
		}
	}
	pathValue := directory
	if currentPath != "" {
		pathValue += string(os.PathListSeparator) + currentPath
	}
	return append(result, "PATH="+pathValue)
}

func copyAssets(pluginDir, artifactRoot string, assets []string, mappedAssets []AssetMapping) error {
	for _, asset := range assets {
		if err := copyAsset(pluginDir, artifactRoot, asset, asset); err != nil {
			return err
		}
	}
	for _, asset := range mappedAssets {
		if err := copyAsset(pluginDir, artifactRoot, asset.Source, asset.Destination); err != nil {
			return err
		}
	}
	return nil
}

func copyAsset(pluginDir, artifactRoot, sourcePath, destinationPath string) error {
	sourceClean, err := cleanRelativePath(sourcePath)
	if err != nil {
		return fmt.Errorf("pluginbuild: asset source path %q: %w", sourcePath, err)
	}
	destinationClean, err := cleanRelativePath(destinationPath)
	if err != nil {
		return fmt.Errorf("pluginbuild: asset destination path %q: %w", destinationPath, err)
	}
	source := filepath.Join(pluginDir, sourceClean)
	if !pathWithin(pluginDir, source) {
		return fmt.Errorf("pluginbuild: asset source path %q escapes the plugin directory", sourcePath)
	}
	destination := filepath.Join(artifactRoot, destinationClean)
	if !pathWithin(artifactRoot, destination) {
		return fmt.Errorf("pluginbuild: asset destination path %q escapes the artifact root", destinationPath)
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("pluginbuild: inspect asset %q: %w", sourcePath, err)
	}
	if info.IsDir() {
		return copyTree(source, destination)
	}
	return copyFile(source, destination, 0o644)
}

func cleanRelativePath(value string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(value)))
	if clean == "." || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path must stay below its root")
	}
	return clean, nil
}

func copyLicense(pluginDir, artifactRoot string) error {
	current := pluginDir
	for {
		candidate := filepath.Join(current, "LICENSE")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return copyFile(candidate, filepath.Join(artifactRoot, "LICENSE"), 0o644)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return errors.New("pluginbuild: LICENSE not found in plugin or parent directories")
}

func inventory(root string, manifest Manifest, platform string, infoBytes []byte) (Artifact, error) {
	files := make([]ArtifactFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "artifact.json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		digest, err := fileSHA256(path)
		if err != nil {
			return err
		}
		files = append(files, ArtifactFile{
			Path:   relative,
			Role:   fileRole(relative, manifest.Entry, platform),
			Size:   info.Size(),
			SHA256: digest,
		})
		return nil
	})
	if err != nil {
		return Artifact{}, fmt.Errorf("pluginbuild: inventory artifact: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	manifestDigest := sha256.Sum256(infoBytes)
	return Artifact{
		ArtifactVersion: ArtifactVersion,
		PluginID:        manifest.ID,
		PluginVersion:   manifest.Version,
		TargetPlatform:  platform,
		ManifestSHA256:  hex.EncodeToString(manifestDigest[:]),
		Files:           files,
	}, nil
}

func fileRole(path, logicalEntry, platform string) string {
	switch {
	case path == "info.json":
		return "manifest"
	case path == logicalEntry || path == logicalEntry+".exe":
		return "backend"
	case strings.HasPrefix(path, "ui/"):
		return "ui"
	case strings.HasPrefix(path, "templates/"):
		return "render_template"
	case path == "LICENSE":
		return "license"
	case strings.Contains(strings.ToUpper(filepath.Base(path)), "NOTICE"):
		return "notice"
	case strings.HasSuffix(path, ".spdx.json"):
		return "sbom"
	default:
		return "data"
	}
}

func writeDeterministicZIP(output, stagingRoot, pluginID string) error {
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("pluginbuild: create archive: %w", err)
	}
	writer := zip.NewWriter(file)
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
			_ = file.Close()
		}
	}()
	root := filepath.Join(stagingRoot, pluginID)
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		relative, _ := filepath.Rel(stagingRoot, path)
		name := filepath.ToSlash(relative)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		if strings.Contains(name, "/bin/") {
			header.SetMode(0o755)
		} else {
			header.SetMode(0o644)
		}
		destination, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(destination, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("pluginbuild: close archive: %w", err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pluginbuild: symbolic links are not allowed in assets: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		return copyFile(path, targetPath, mode)
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, mode)
}

func replaceTree(source, destination string) error {
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	temporary := destination + ".new"
	if err := os.RemoveAll(temporary); err != nil {
		return err
	}
	if err := copyTree(source, temporary); err != nil {
		return err
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		if copyErr := copyTree(temporary, destination); copyErr != nil {
			return fmt.Errorf("publish expanded artifact: %v (copy fallback: %w)", err, copyErr)
		}
		return os.RemoveAll(temporary)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
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

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func writeNotices(ctx context.Context, config Config, pluginDir, root string) error {
	modules, err := goModules(ctx, config, pluginDir)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	buffer.WriteString("# Third-party notices\n\n")
	if len(modules) == 0 {
		buffer.WriteString("This plugin has no third-party Go module dependencies.\n")
	} else {
		for _, module := range modules {
			fmt.Fprintf(&buffer, "- %s %s\n", module.Path, module.Version)
		}
	}
	if uiDependencies, err := readUIDependencies(pluginDir); err != nil {
		return err
	} else if len(uiDependencies) > 0 {
		buffer.WriteString("\n## Vue UI packages\n\n")
		for _, dependency := range uiDependencies {
			fmt.Fprintf(&buffer, "- %s %s\n", dependency.Name, dependency.Version)
		}
	}
	return os.WriteFile(filepath.Join(root, "THIRD_PARTY_NOTICES.md"), buffer.Bytes(), 0o644)
}
