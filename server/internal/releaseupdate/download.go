package releaseupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
}

type Downloader struct {
	HTTPClient    *http.Client
	IdleTimeout   time.Duration
	TotalTimeout  time.Duration
	FreeDiskBytes func(string) (uint64, error)
}

func NewDownloader() *Downloader {
	return &Downloader{
		HTTPClient:    newSecureHTTPClient(30 * time.Minute),
		IdleTimeout:   30 * time.Second,
		TotalTimeout:  30 * time.Minute,
		FreeDiskBytes: freeDiskBytes,
	}
}

func (d *Downloader) Download(ctx context.Context, check CheckResult, destinationRoot string, progress func(DownloadProgress)) (DownloadedBundle, error) {
	if check.Status != "update_available" {
		return DownloadedBundle{}, errorWithCode(CodeUpdateNotSupported, "download update", errors.New("no newer trusted release is available"))
	}
	if err := validateArtifact(check.Artifact); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "validate artifact metadata", err)
	}
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "create update download directory", err)
	}
	requiredBytes := check.Artifact.ArchiveSizeBytes + check.Artifact.ExpandedSizeBytes
	freeDisk := d.FreeDiskBytes
	if freeDisk == nil {
		freeDisk = freeDiskBytes
	}
	if freeBytes, err := freeDisk(destinationRoot); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeDiskSpaceInsufficient, "inspect update disk space", err)
	} else if freeBytes < uint64(requiredBytes) {
		return DownloadedBundle{}, errorWithCode(CodeDiskSpaceInsufficient, "reserve update disk space", fmt.Errorf("need %d bytes, have %d bytes", requiredBytes, freeBytes))
	}

	artifactPath := filepath.Join(destinationRoot, check.Artifact.FileName)
	if info, err := os.Stat(artifactPath); err == nil && !info.IsDir() {
		if verifyErr := VerifyArtifactFile(artifactPath, check.Artifact); verifyErr == nil {
			return DownloadedBundle{CheckResult: check, ArtifactPath: artifactPath}, nil
		}
		if err := os.Remove(artifactPath); err != nil {
			return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "remove invalid cached artifact", err)
		}
	}
	partialPath := artifactPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "remove stale partial artifact", err)
	}

	downloadURL, err := artifactDownloadURL(check.AvailableVersion, check.Artifact.FileName)
	if err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "build artifact URL", err)
	}
	totalTimeout := d.TotalTimeout
	if totalTimeout <= 0 {
		totalTimeout = 30 * time.Minute
	}
	downloadContext, cancelTotal := context.WithTimeout(ctx, totalTimeout)
	defer cancelTotal()
	idleTimeout := d.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second
	}
	idleContext, cancelIdle := context.WithCancel(downloadContext)
	defer cancelIdle()
	idleTimer := time.AfterFunc(idleTimeout, cancelIdle)
	defer idleTimer.Stop()

	request, err := http.NewRequestWithContext(idleContext, http.MethodGet, downloadURL, nil)
	if err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "create artifact request", err)
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "RayleaBot-Updater/2")
	client := d.HTTPClient
	if client == nil {
		client = newSecureHTTPClient(totalTimeout)
	}
	response, err := client.Do(request)
	if err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "download artifact", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "download artifact", fmt.Errorf("artifact returned HTTP %d", response.StatusCode))
	}
	if response.ContentLength > check.Artifact.ArchiveSizeBytes {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "download artifact", errors.New("artifact Content-Length exceeds the signed size"))
	}

	partial, err := os.OpenFile(partialPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "create partial artifact", err)
	}
	removePartial := true
	defer func() {
		_ = partial.Close()
		if removePartial {
			_ = os.Remove(partialPath)
		}
	}()

	hash := sha256.New()
	buffer := make([]byte, 1024*1024)
	var downloaded int64
	for {
		readCount, readErr := response.Body.Read(buffer)
		if readCount > 0 {
			if !idleTimer.Stop() {
				select {
				case <-idleContext.Done():
				default:
				}
			}
			idleTimer.Reset(idleTimeout)
			downloaded += int64(readCount)
			if downloaded > check.Artifact.ArchiveSizeBytes || downloaded > MaxArchiveBytes {
				return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "download artifact", errors.New("artifact exceeds the signed size"))
			}
			if _, err := partial.Write(buffer[:readCount]); err != nil {
				return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "write partial artifact", err)
			}
			_, _ = hash.Write(buffer[:readCount])
			if progress != nil {
				progress(DownloadProgress{DownloadedBytes: downloaded, TotalBytes: check.Artifact.ArchiveSizeBytes})
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "read artifact response", readErr)
		}
	}
	if downloaded != check.Artifact.ArchiveSizeBytes {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "verify artifact size", fmt.Errorf("expected %d bytes, received %d", check.Artifact.ArchiveSizeBytes, downloaded))
	}
	if digest := hex.EncodeToString(hash.Sum(nil)); digest != check.Artifact.SHA256 {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "verify artifact digest", errors.New("artifact SHA256 does not match the signed manifest"))
	}
	if err := partial.Sync(); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "sync partial artifact", err)
	}
	if err := partial.Close(); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "close partial artifact", err)
	}
	if err := os.Rename(partialPath, artifactPath); err != nil {
		return DownloadedBundle{}, errorWithCode(CodeArtifactInvalid, "commit downloaded artifact", err)
	}
	removePartial = false
	return DownloadedBundle{CheckResult: check, ArtifactPath: artifactPath}, nil
}

func VerifyArtifactFile(artifactPath string, artifact Artifact) error {
	info, err := os.Stat(artifactPath)
	if err != nil {
		return errorWithCode(CodeArtifactInvalid, "stat artifact", err)
	}
	if !info.Mode().IsRegular() || info.Size() != artifact.ArchiveSizeBytes || info.Size() > MaxArchiveBytes {
		return errorWithCode(CodeArtifactInvalid, "verify artifact size", fmt.Errorf("artifact size is %d, expected %d", info.Size(), artifact.ArchiveSizeBytes))
	}
	file, err := os.Open(artifactPath)
	if err != nil {
		return errorWithCode(CodeArtifactInvalid, "open artifact", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, MaxArchiveBytes+1)); err != nil {
		return errorWithCode(CodeArtifactInvalid, "hash artifact", err)
	}
	if digest := hex.EncodeToString(hash.Sum(nil)); digest != artifact.SHA256 {
		return errorWithCode(CodeArtifactInvalid, "verify artifact digest", errors.New("artifact SHA256 does not match the signed manifest"))
	}
	return nil
}
