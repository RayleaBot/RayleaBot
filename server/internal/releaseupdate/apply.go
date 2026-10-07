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
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
)

const (
	releaseRenameAttempts   = 10
	releaseRenameRetryDelay = 100 * time.Millisecond
)

// Configured distribution routes are trusted. Archive size, CRC/gzip and
// staged build information detect incomplete or mismatched downloads. A failed
// replacement keeps the prepared archive for an entirely local retry.

// Download fetches the installed artifact's archive when a newer release exists
// and returns its cached path.
func (c *Checker) Download(ctx context.Context, installRoot string) (CheckResult, string, error) {
	c = c.configured()
	lock, err := filelock.Acquire(filepath.Join(installRoot, "cache", "update", "operation.lock"))
	if err != nil {
		return CheckResult{}, "", err
	}
	defer lock.Close()
	result, err := c.Check(ctx, installRoot)
	if err != nil || result.Status != "update_available" {
		if err == nil {
			err = discardPrepared(installRoot)
		}
		return result, "", err
	}
	c.report("probe", "", 0, 0)
	routes := c.downloadRoutes(ctx, result.Artifact, result.AvailableVersion)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		for _, raw := range routes {
			if err := ctx.Err(); err != nil {
				return result, "", err
			}
			artifact := result.Artifact
			artifact.DownloadURL = raw
			archivePath, err := c.ensureArchive(ctx, installRoot, artifact)
			if err != nil {
				if ctx.Err() != nil || localFileError(err) {
					return result, "", err
				}
				lastErr = err
				continue
			}
			prepared, err := c.prepareArchive(ctx, installRoot, archivePath, result)
			if err == nil {
				return prepared, archivePath, nil
			}
			var invalid *invalidArchiveError
			if ctx.Err() != nil || !errors.As(err, &invalid) {
				return result, "", err
			}
			lastErr = err
			if err := os.Remove(archivePath); err != nil {
				return result, "", err
			}
		}
	}
	return result, "", fmt.Errorf("all update download routes failed: %w", lastErr)
}

// Switching network routes cannot repair a local filesystem failure.
func localFileError(err error) bool {
	var pathError *os.PathError
	var linkError *os.LinkError
	return errors.As(err, &pathError) || errors.As(err, &linkError)
}

type invalidArchiveError struct{ cause error }

func (e *invalidArchiveError) Error() string { return e.cause.Error() }
func (e *invalidArchiveError) Unwrap() error { return e.cause }

// Apply installs a newer release over installRoot file by file. The caller must
// hold the service lifecycle lock.
func (c *Checker) Apply(ctx context.Context, installRoot string) (CheckResult, error) {
	return c.ApplyPrepared(ctx, installRoot, "")
}

func (c *Checker) ApplyPrepared(ctx context.Context, installRoot, expectedID string) (CheckResult, error) {
	lock, err := filelock.Acquire(filepath.Join(installRoot, "cache", "update", "operation.lock"))
	if err != nil {
		return CheckResult{}, err
	}
	defer lock.Close()
	prepared, err := readPrepared(installRoot)
	if err != nil {
		return CheckResult{}, fmt.Errorf("prepare an update with update download first: %w", err)
	}
	result := prepared.Result
	if expectedID != "" && prepared.Staging != expectedID {
		return result, errors.New("prepared update was replaced; prepare the selected version again")
	}
	current := InstalledVersion(installRoot)
	if current == result.AvailableVersion {
		result.Status = "up_to_date"
		return result, nil
	}
	if current != result.CurrentVersion {
		return result, errors.New("installation changed since the update was prepared")
	}
	replacedRoot := filepath.Join(installRoot, "cache", "update", "replaced")
	// Windows keeps executables replaced by the previous update until they exit.
	_ = os.RemoveAll(replacedRoot)
	archivePath := filepath.Join(installRoot, "cache", "downloads", "update", result.Artifact.FileName)
	staging := filepath.Join(installRoot, "cache", "update", prepared.Staging)
	// Re-extract on retries because successful moves consume staging files.
	// This is a local operation, and never refreshes the selected release.
	if prepared.Applying {
		if err := os.RemoveAll(staging); err != nil {
			return result, err
		}
	}
	var releaseRoot string
	if prepared.Applying {
		releaseRoot, err = stageRelease(ctx, archivePath, staging, result)
	} else {
		releaseRoot = filepath.Join(staging, "RayleaBot-v"+result.AvailableVersion+"-"+result.Artifact.ArtifactID)
		err = validateStagedRelease(releaseRoot, result)
	}
	if err != nil {
		return result, err
	}
	prepared.Applying = true
	if err := writePrepared(installRoot, prepared); err != nil {
		return result, err
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
	_ = os.Remove(filepath.Join(installRoot, "cache", "update", "prepared.json"))
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
	c.report("download", artifact.DownloadURL, 0, artifact.ArchiveSizeBytes)
	if err := deps.DownloadHTTPSWithProgress(ctx, c.DownloadClient, artifact.DownloadURL, partial, artifact.ArchiveSizeBytes, func(p deps.DownloadProgress) {
		c.report("download", artifact.DownloadURL, p.DownloadedBytes, artifact.ArchiveSizeBytes)
	}); err != nil {
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
		if ctx.Err() != nil || localFileError(err) {
			return "", err
		}
		return "", &invalidArchiveError{cause: fmt.Errorf("extract release archive: %w", err)}
	}
	releaseRoot := filepath.Join(staging, "RayleaBot-v"+result.AvailableVersion+"-"+result.Artifact.ArtifactID)
	if err := validateStagedRelease(releaseRoot, result); err != nil {
		if localFileError(err) && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", &invalidArchiveError{cause: err}
	}
	return releaseRoot, nil
}

func validateStagedRelease(releaseRoot string, result CheckResult) error {
	payload, err := os.ReadFile(filepath.Join(releaseRoot, "build_info.json"))
	if err != nil {
		return fmt.Errorf("read staged build_info.json: %w", err)
	}
	staged, err := DecodeBuildInfo(payload)
	if err != nil {
		return err
	}
	if staged.Version != result.AvailableVersion || staged.ArtifactID != result.Artifact.ArtifactID {
		return errors.New("staged release does not match the release manifest")
	}
	return filepath.WalkDir(releaseRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(releaseRoot, path)
		if err != nil {
			return err
		}
		first := strings.Split(filepath.ToSlash(relative), "/")[0]
		switch strings.ToLower(first) {
		case "config", "data", "plugins", "logs", "backups", "cache":
			return fmt.Errorf("release contains protected path %s", first)
		}
		if strings.HasPrefix(strings.ToLower(filepath.ToSlash(relative)), ".deps/store") {
			return errors.New("release contains managed runtime store")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("release contains symlink")
		}
		return nil
	})
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
