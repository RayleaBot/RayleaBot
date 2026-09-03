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
	ArtifactVersion           = "2"
	ManifestVersion           = "3"
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
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	ManifestVersion string `json:"manifest_version"`
	MinCoreVersion  string `json:"min_core_version"`
	License         string `json:"license"`
	ManagementUI    *struct {
		Entry string `json:"entry"`
		Pages []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"pages"`
	} `json:"management_ui,omitempty"`
}

type Artifact struct {
	ArtifactVersion string `json:"artifact_version"`
	TargetPlatform  string `json:"target_platform"`
	Entry           string `json:"entry"`
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
	backendPackage := strings.TrimSpace(config.BackendPackage)
	if backendPackage == "" {
		backendPackage, err = inferBackendPackage(pluginDir, manifest.ID)
		if err != nil {
			return Result{}, err
		}
	}
	backendPackage, err = resolveBackendPackage(pluginDir, backendPackage)
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

	entry := filepath.ToSlash(filepath.Join("bin", manifest.ID+target.EXE))
	binaryPath := filepath.Join(root, filepath.FromSlash(entry))
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
	if err := copyStandardAssets(pluginDir, root); err != nil {
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
	return finalizeArtifact(root, staging, outputDir, manifest, entry, config.TargetPlatform, config.KeepExpandedArtifact)
}

func copyStandardAssets(pluginDir, artifactRoot string) error {
	for _, name := range []string{"assets", "templates", "LICENSES"} {
		if _, err := os.Stat(filepath.Join(pluginDir, name)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := copyAsset(pluginDir, artifactRoot, name, name); err != nil {
			return err
		}
	}
	return nil
}

func finalizeArtifact(root, staging, outputDir string, manifest Manifest, entry, platform string, keepExpanded bool) (Result, error) {
	artifact, err := inventory(root, entry, platform)
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
	archivePath := filepath.Join(outputDir, fmt.Sprintf("%s-%s-%s.zip", manifest.ID, manifest.Version, platform))
	if err := writeDeterministicZIP(archivePath, staging, manifest.ID); err != nil {
		return Result{}, err
	}
	archiveDigest, err := fileSHA256(archivePath)
	if err != nil {
		return Result{}, err
	}
	artifactDir := ""
	if keepExpanded {
		artifactDir = filepath.Join(outputDir, platform, manifest.ID)
		if err := replaceTree(root, artifactDir); err != nil {
			return Result{}, err
		}
	}
	return Result{
		PluginID:       manifest.ID,
		Version:        manifest.Version,
		TargetPlatform: platform,
		ArtifactDir:    artifactDir,
		ArchivePath:    archivePath,
		ArchiveSHA256:  archiveDigest,
	}, nil
}

func validateManifest(manifest Manifest, platform string) error {
	if manifest.ID == "" || manifest.Name == "" || manifest.Version == "" || manifest.MinCoreVersion == "" || manifest.License == "" {
		return errors.New("pluginbuild: manifest id, name, version, license and min_core_version are required")
	}
	if manifest.ManifestVersion != ManifestVersion {
		return fmt.Errorf("pluginbuild: manifest_version must be %s", ManifestVersion)
	}
	if strings.TrimSpace(platform) == "" {
		return nil
	}
	_, err := resolveTarget(platform)
	return err
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

func inferBackendPackage(pluginDir, pluginID string) (string, error) {
	cmdDir := filepath.Join(pluginDir, "cmd")
	candidates := []string{pluginID}
	if index := strings.LastIndex(pluginID, "."); index >= 0 && index+1 < len(pluginID) {
		candidates = append(candidates, pluginID[index+1:])
	}
	for _, name := range candidates {
		path := filepath.Join(cmdDir, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return filepath.ToSlash(filepath.Join("cmd", name)), nil
		}
	}
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ".", nil
		}
		return "", fmt.Errorf("pluginbuild: inspect cmd directory: %w", err)
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}
	if len(directories) == 1 {
		return filepath.ToSlash(filepath.Join("cmd", directories[0])), nil
	}
	if len(directories) > 1 {
		return "", errors.New("pluginbuild: multiple Go commands found; select one with --backend")
	}
	return ".", nil
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
	source := filepath.Clean(filepath.FromSlash(strings.TrimSpace(sourcePath)))
	if !filepath.IsAbs(source) {
		sourceClean, err := cleanRelativePath(sourcePath)
		if err != nil {
			return fmt.Errorf("pluginbuild: asset source path %q: %w", sourcePath, err)
		}
		source = filepath.Join(pluginDir, sourceClean)
	}
	destinationClean, err := cleanRelativePath(destinationPath)
	if err != nil {
		return fmt.Errorf("pluginbuild: asset destination path %q: %w", destinationPath, err)
	}
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

func inventory(_ string, entryPath, platform string) (Artifact, error) {
	return Artifact{
		ArtifactVersion: ArtifactVersion,
		TargetPlatform:  platform,
		Entry:           entryPath,
	}, nil
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
