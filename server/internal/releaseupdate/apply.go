package releaseupdate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
)

const (
	releaseRenameAttempts   = 10
	releaseRenameRetryDelay = 100 * time.Millisecond
)

// Updates rely on HTTPS, the manifest archive size and the archive's own CRC or
// gzip checksums. A digest published beside the archive could not stop
// tampering, so none is checked. A failed replacement is not rolled back:
// build_info.json is written last, the installation keeps reporting the old
// version, and apply can run again.

// Download fetches the installed artifact's archive when a newer release exists
// and returns its cached path.
func (c *Checker) Download(ctx context.Context, installRoot string) (CheckResult, string, error) {
	result, err := c.Check(ctx, installRoot)
	if err != nil || result.Status != "update_available" {
		return result, "", err
	}
	archivePath, err := c.ensureArchive(ctx, installRoot, result.Artifact)
	return result, archivePath, err
}

// Apply installs a newer release over installRoot file by file. The caller must
// hold the service lifecycle lock.
func (c *Checker) Apply(ctx context.Context, installRoot string) (CheckResult, error) {
	replacedRoot := filepath.Join(installRoot, "cache", "update", "replaced")
	// Windows keeps executables replaced by the previous update until they exit.
	_ = os.RemoveAll(replacedRoot)
	result, archivePath, err := c.Download(ctx, installRoot)
	if err != nil || result.Status != "update_available" {
		return result, err
	}
	staging := filepath.Join(installRoot, "cache", "update", "staging")
	if err := os.RemoveAll(staging); err != nil {
		return result, err
	}
	releaseRoot, err := stageRelease(ctx, archivePath, staging, result)
	if err != nil {
		return result, errors.Join(err, os.Remove(archivePath))
	}
	if err := os.MkdirAll(replacedRoot, 0o755); err != nil {
		return result, err
	}
	// A per-run directory keeps a still-running executable from an earlier
	// attempt from blocking the move of the same path.
	replaced, err := os.MkdirTemp(replacedRoot, "run-")
	if err != nil {
		return result, err
	}
	if err := installRelease(ctx, installRoot, releaseRoot, replaced); err != nil {
		return result, fmt.Errorf("install release files; run update apply again or extract the release manually: %w", err)
	}
	_ = os.RemoveAll(staging)
	_ = os.Remove(archivePath)
	_ = os.RemoveAll(replacedRoot)
	return result, nil
}

func (c *Checker) ensureArchive(ctx context.Context, installRoot string, artifact Artifact) (string, error) {
	if err := validateHTTPSURL(artifact.DownloadURL); err != nil {
		return "", errorWithCode(CodeManifestInvalid, "validate download_url", err)
	}
	directory := filepath.Join(installRoot, "cache", "downloads", "update")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(directory, artifact.FileName)
	if info, err := os.Stat(archivePath); err == nil && info.Mode().IsRegular() && info.Size() == artifact.ArchiveSizeBytes {
		return archivePath, nil
	} else if err == nil {
		if err := os.RemoveAll(archivePath); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	partial := archivePath + ".part"
	if err := os.Remove(partial); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := deps.DownloadHTTPS(ctx, c.DownloadClient, artifact.DownloadURL, partial, artifact.ArchiveSizeBytes); err != nil {
		return "", fmt.Errorf("download release archive: %w", err)
	}
	info, err := os.Stat(partial)
	if err != nil {
		return "", err
	}
	if info.Size() != artifact.ArchiveSizeBytes {
		return "", errors.Join(fmt.Errorf("release archive has %d bytes, manifest lists %d", info.Size(), artifact.ArchiveSizeBytes), os.Remove(partial))
	}
	if err := os.Rename(partial, archivePath); err != nil {
		return "", err
	}
	return archivePath, nil
}

func stageRelease(ctx context.Context, archivePath, staging string, result CheckResult) (string, error) {
	var format string
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		format = "zip"
	case strings.HasSuffix(archivePath, ".tar.gz"):
		format = "tar.gz"
	default:
		return "", fmt.Errorf("unsupported release archive %s", filepath.Base(archivePath))
	}
	if err := deps.ExtractWithProgress(ctx, archivePath, format, staging, nil); err != nil {
		return "", fmt.Errorf("extract release archive: %w", err)
	}
	releaseRoot := filepath.Join(staging, "RayleaBot-v"+result.AvailableVersion+"-"+result.Artifact.ArtifactID)
	payload, err := os.ReadFile(filepath.Join(releaseRoot, "build_info.json"))
	if err != nil {
		return "", fmt.Errorf("read staged build_info.json: %w", err)
	}
	staged, err := DecodeBuildInfo(payload)
	if err != nil {
		return "", err
	}
	if staged.Version != result.AvailableVersion || staged.ArtifactID != result.Artifact.ArtifactID {
		return "", errors.New("staged release does not match the release manifest")
	}
	return releaseRoot, nil
}

func installRelease(ctx context.Context, installRoot, releaseRoot, replaced string) error {
	var files []string
	err := filepath.WalkDir(releaseRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(releaseRoot, path)
		if err != nil {
			return err
		}
		if relative != "build_info.json" {
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, relative := range append(files, "build_info.json") {
		if err := installReleaseFile(ctx, installRoot, releaseRoot, replaced, relative); err != nil {
			return fmt.Errorf("%s: %w", filepath.ToSlash(relative), err)
		}
	}
	return nil
}

// installReleaseFile moves an existing file aside before moving the new one in;
// renaming works for a running executable on Windows where overwriting does not.
func installReleaseFile(ctx context.Context, installRoot, releaseRoot, replaced, relative string) error {
	target := filepath.Join(installRoot, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil {
		if info.IsDir() {
			return errors.New("installation path is a directory")
		}
		aside := filepath.Join(replaced, relative)
		if err := os.MkdirAll(filepath.Dir(aside), 0o755); err != nil {
			return err
		}
		if err := renameReleaseFile(ctx, target, aside); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return renameReleaseFile(ctx, filepath.Join(releaseRoot, relative), target)
}

func renameReleaseFile(ctx context.Context, source, target string) error {
	return fsguard.RenameWithRetry(ctx, source, target, fsguard.RenameRetryOptions{
		Attempts: releaseRenameAttempts,
		Delay:    releaseRenameRetryDelay,
	})
}
