package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const ManifestVersion = 2

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Manifest struct {
	ManifestVersion int        `json:"manifest_version"`
	Resources       []Resource `json:"resources"`
}

type Resource struct {
	ID            string              `json:"id"`
	Kind          string              `json:"kind"`
	Version       string              `json:"version"`
	Platform      string              `json:"platform"`
	Source        string              `json:"source"`
	SHA256        string              `json:"sha256"`
	ArchiveFormat string              `json:"archive_format"`
	Entrypoints   map[string][]string `json:"entrypoints"`
}

type PreparedResource struct {
	Resource    Resource
	Root        string
	Entrypoints map[string]string
}

type Manager struct {
	repoRoot     string
	downloadFile func(context.Context, string, string) error
	extract      func(context.Context, string, string, string) error
	now          func() time.Time
}

func NewManager(repoRoot string) *Manager {
	return &Manager{
		repoRoot:     strings.TrimSpace(repoRoot),
		downloadFile: downloadHTTPSFile,
		extract:      extractArchive,
		now:          time.Now,
	}
}

func LoadManifest(repoRoot string) (*Manifest, error) {
	return LoadManifestPath(filepath.Join(strings.TrimSpace(repoRoot), ".deps", "manifest.json"))
}

func LoadManifestPath(manifestPath string) (*Manifest, error) {
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode deps manifest: %w", err)
	}
	if manifest.ManifestVersion != ManifestVersion {
		return nil, fmt.Errorf("unsupported deps manifest version %d", manifest.ManifestVersion)
	}
	return &manifest, nil
}

func CurrentPlatform() string {
	return ManifestPlatform(runtime.GOOS, runtime.GOARCH)
}

func ManifestPlatform(goos, goarch string) string {
	switch goos {
	case "windows":
		return "windows-" + normalizeManifestArch(goarch)
	case "darwin":
		return "macos-" + normalizeManifestArch(goarch)
	default:
		return goos + "-" + normalizeManifestArch(goarch)
	}
}

func normalizeManifestArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	default:
		return goarch
	}
}

func (manifest *Manifest) HasPlatform(platform string) bool {
	if manifest == nil {
		return false
	}
	for _, resource := range manifest.Resources {
		if resource.Platform == platform {
			return true
		}
	}
	return false
}

func (manifest *Manifest) FindResource(platform, kind string) *Resource {
	if manifest == nil {
		return nil
	}
	for i := range manifest.Resources {
		resource := &manifest.Resources[i]
		if resource.Platform == platform && resource.Kind == kind {
			return resource
		}
	}
	return nil
}

func ResourceMetadataComplete(resource *Resource) bool {
	if resource == nil {
		return false
	}
	if strings.TrimSpace(resource.ArchiveFormat) == "" {
		return false
	}
	if !archiveFormatSupported(resource.ArchiveFormat) {
		return false
	}
	if !resourceHasRequiredEntrypoints(resource) {
		return false
	}
	source := strings.TrimSpace(resource.Source)
	if source == "" || strings.Contains(strings.ToUpper(source), "TODO(") {
		return false
	}
	parsedURL, err := url.Parse(source)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return false
	}
	sha256 := strings.ToLower(strings.TrimSpace(resource.SHA256))
	if strings.Contains(strings.ToUpper(sha256), "TODO(") {
		return false
	}
	return sha256Pattern.MatchString(sha256)
}

func archiveFormatSupported(format string) bool {
	switch strings.TrimSpace(format) {
	case "zip", "tar.gz", "tar.xz":
		return true
	default:
		return false
	}
}

func resourceHasRequiredEntrypoints(resource *Resource) bool {
	required := requiredEntrypoints(resource)
	if len(required) == 0 {
		return false
	}
	if len(resource.Entrypoints) == 0 {
		return false
	}
	for _, key := range required {
		candidates := resource.Entrypoints[key]
		if len(candidates) == 0 {
			return false
		}
		valid := false
		for _, candidate := range candidates {
			clean := strings.TrimSpace(candidate)
			if clean == "" {
				continue
			}
			if filepath.IsAbs(clean) {
				continue
			}
			if clean == "." || strings.HasPrefix(clean, "..") {
				continue
			}
			valid = true
			break
		}
		if !valid {
			return false
		}
	}
	return true
}

func requiredEntrypoints(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	switch resource.Kind {
	case "chromium":
		return []string{"browser"}
	case "python-runtime":
		return []string{"python", "pip"}
	case "nodejs-runtime":
		return []string{"node", "npm"}
	default:
		return nil
	}
}

func StoreRoot(repoRoot string, resource *Resource) string {
	if resource == nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(repoRoot), ".deps", "store", resource.ID, resource.Version)
}

func CacheRoot(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "runtime")
}

func LockPath(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "platform.lock")
}

func (m *Manager) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	prepared, err := m.resolvePreparedResource(kind)
	if err != nil {
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) ResolveEntrypoint(ctx context.Context, kind, name string) (string, error) {
	prepared, err := m.Prepare(ctx, kind)
	if err != nil {
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) Prepare(ctx context.Context, kind string) (*PreparedResource, error) {
	if m == nil {
		return nil, errors.New("deps manager is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	prepared, err := m.resolvePreparedResource(kind)
	if err == nil {
		return prepared, nil
	}

	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		return nil, err
	}
	if !manifest.HasPlatform(CurrentPlatform()) {
		return nil, fmt.Errorf("deps manifest does not include current platform %s", CurrentPlatform())
	}
	if !ResourceMetadataComplete(resource) {
		return nil, fmt.Errorf("deps resource %s for %s is not bootstrap-ready", kind, CurrentPlatform())
	}

	lockPath := LockPath(m.repoRoot)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create deps lock root: %w", err)
	}
	release, err := acquireLock(ctx, lockPath, m.now)
	if err != nil {
		return nil, err
	}
	defer release()

	if prepared, err := m.resolvePreparedManifestResource(manifest, resource); err == nil {
		return prepared, nil
	}

	if err := os.MkdirAll(CacheRoot(m.repoRoot), 0o755); err != nil {
		return nil, fmt.Errorf("create deps cache root: %w", err)
	}
	archivePath := filepath.Join(CacheRoot(m.repoRoot), resource.ID+"-"+resource.Version+archiveSuffix(resource.ArchiveFormat))
	if err := ensureDownloadedArchive(ctx, archivePath, resource, m.downloadFile); err != nil {
		return nil, err
	}
	if err := ensurePreparedResource(ctx, m.repoRoot, *resource, archivePath, m.extract); err != nil {
		return nil, err
	}

	return m.resolvePreparedManifestResource(manifest, resource)
}

func (m *Manager) resolvePreparedResource(kind string) (*PreparedResource, error) {
	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		return nil, err
	}
	return m.resolvePreparedManifestResource(manifest, resource)
}

func (m *Manager) resolvePreparedManifestResource(_ *Manifest, resource *Resource) (*PreparedResource, error) {
	storeRoot := StoreRoot(m.repoRoot, resource)
	entrypoints, err := resolvePreparedEntrypoints(storeRoot, resource)
	if err != nil {
		return nil, err
	}
	return &PreparedResource{
		Resource:    *resource,
		Root:        storeRoot,
		Entrypoints: entrypoints,
	}, nil
}

func (m *Manager) currentResource(kind string) (*Manifest, *Resource, error) {
	manifest, err := LoadManifest(m.repoRoot)
	if err != nil {
		return nil, nil, err
	}
	resource := manifest.FindResource(CurrentPlatform(), kind)
	if resource == nil {
		return manifest, nil, fmt.Errorf("deps manifest does not include %s for %s", kind, CurrentPlatform())
	}
	return manifest, resource, nil
}

func resolvePreparedEntrypoints(storeRoot string, resource *Resource) (map[string]string, error) {
	if resource == nil {
		return nil, errors.New("deps resource is required")
	}
	entrypoints := make(map[string]string, len(resource.Entrypoints))
	for _, key := range requiredEntrypoints(resource) {
		candidates := resource.Entrypoints[key]
		var resolved string
		for _, candidate := range candidates {
			clean := filepath.Clean(filepath.Join(storeRoot, filepath.FromSlash(candidate)))
			if !pathWithinRoot(storeRoot, clean) {
				continue
			}
			info, err := os.Stat(clean)
			if err != nil || info.IsDir() {
				continue
			}
			resolved = clean
			break
		}
		if resolved == "" {
			return nil, fmt.Errorf("prepared deps resource %s is missing entrypoint %s", resource.Kind, key)
		}
		entrypoints[key] = resolved
	}
	return entrypoints, nil
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func ensureDownloadedArchive(ctx context.Context, archivePath string, resource *Resource, downloader func(context.Context, string, string) error) error {
	if err := verifyFileSHA256(archivePath, resource.SHA256); err == nil {
		return nil
	}
	tempPath := archivePath + ".download"
	_ = os.Remove(tempPath)
	if err := downloader(ctx, resource.Source, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("download deps resource %s: %w", resource.Kind, err)
	}
	if err := verifyFileSHA256(tempPath, resource.SHA256); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("verify deps resource %s archive: %w", resource.Kind, err)
	}
	if err := os.Rename(tempPath, archivePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("persist deps archive %s: %w", resource.Kind, err)
	}
	return nil
}

func ensurePreparedResource(
	ctx context.Context,
	repoRoot string,
	resource Resource,
	archivePath string,
	extractor func(context.Context, string, string, string) error,
) error {
	storeRoot := StoreRoot(repoRoot, &resource)
	if _, err := resolvePreparedEntrypoints(storeRoot, &resource); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(storeRoot), 0o755); err != nil {
		return fmt.Errorf("create deps store root: %w", err)
	}
	tempRoot, err := os.MkdirTemp(filepath.Dir(storeRoot), "."+resource.ID+"-"+resource.Version+"-*")
	if err != nil {
		return fmt.Errorf("create deps temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	if err := extractor(ctx, archivePath, resource.ArchiveFormat, tempRoot); err != nil {
		return fmt.Errorf("extract deps resource %s: %w", resource.Kind, err)
	}
	_ = os.RemoveAll(storeRoot)
	if err := os.Rename(tempRoot, storeRoot); err != nil {
		return fmt.Errorf("activate deps resource %s: %w", resource.Kind, err)
	}
	return nil
}

func archiveSuffix(format string) string {
	switch format {
	case "tar.gz":
		return ".tar.gz"
	case "tar.xz":
		return ".tar.xz"
	default:
		return ".zip"
	}
}

func verifyFileSHA256(path string, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if strings.ToLower(strings.TrimSpace(want)) != got {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, want)
	}
	return nil
}

func acquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = io.WriteString(file, fmt.Sprintf("%d %s\n", os.Getpid(), now().UTC().Format(time.RFC3339)))
			_ = file.Close()
			return func() {
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire deps lock: %w", err)
		}
		info, statErr := os.Stat(path)
		if statErr == nil && now().Sub(info.ModTime()) > 30*time.Minute {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}

func extractArchive(ctx context.Context, archivePath, archiveFormat, destRoot string) error {
	switch archiveFormat {
	case "zip":
		return extractZip(archivePath, destRoot)
	case "tar.gz":
		return extractTarGz(archivePath, destRoot)
	case "tar.xz":
		cmd := exec.CommandContext(ctx, "tar", "-xf", archivePath, "-C", destRoot)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if len(output) == 0 {
				return err
			}
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	default:
		return fmt.Errorf("unsupported archive format %s", archiveFormat)
	}
}

func extractZip(archivePath, destRoot string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath := filepath.Join(destRoot, filepath.FromSlash(file.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		out.Close()
		in.Close()
	}
	return nil
}

func extractTarGz(archivePath, destRoot string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	reader := tar.NewReader(gzr)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destRoot, filepath.FromSlash(header.Name))
		if !pathWithinRoot(destRoot, targetPath) {
			return fmt.Errorf("tar entry escapes destination: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}
