package deps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const abandonedLockAge = 30 * time.Minute

func VerifyFileSHA256(path string, want string) error {
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

func AcquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	return acquireLockWithOwnerCheck(ctx, path, now, processIsRunning)
}

func acquireLockWithOwnerCheck(ctx context.Context, path string, now func() time.Time, ownerIsRunning func(int) bool) (func(), error) {
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
		if lockCanBeReclaimed(path, now(), ownerIsRunning) {
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

func lockCanBeReclaimed(path string, now time.Time, ownerIsRunning func(int) bool) bool {
	if ownerPID, ok := readLockOwnerPID(path); ok && ownerPID != os.Getpid() && ownerIsRunning != nil && !ownerIsRunning(ownerPID) {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && now.Sub(info.ModTime()) > abandonedLockAge
}

func readLockOwnerPID(path string) (int, bool) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return 0, false
	}
	pid, err := strconv.Atoi(fields[0])
	return pid, err == nil && pid > 0
}

func ensurePreparedResourceWithProgress(
	ctx context.Context,
	repoRoot string,
	resource Resource,
	archivePath string,
	extractor func(context.Context, string, string, string) error,
	reporter PrepareProgressReporter,
) error {
	storeRoot := StoreRoot(repoRoot, &resource)
	if _, err := resolvePreparedEntrypoints(storeRoot, &resource); err == nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:    "extract",
			Status:   "succeeded",
			Progress: 100,
			Summary:  managedResourceLabel(resource.Kind) + "已解压",
		}.withResource(&resource, archivePath, storeRoot))
		return nil
	} else if _, statErr := os.Stat(storeRoot); statErr == nil {
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:   "cleanup",
			Status:  "running",
			Summary: "正在清理未完成的 " + managedResourceLabel(resource.Kind) + "目录",
		}.withResource(&resource, archivePath, storeRoot))
		if removeErr := os.RemoveAll(storeRoot); removeErr != nil {
			return fmt.Errorf("clean incomplete deps store root: %w", removeErr)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect deps store root: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(storeRoot), 0o755); err != nil {
		return fmt.Errorf("create deps store root: %w", err)
	}
	if err := removeStaleTempRoots(filepath.Dir(storeRoot), resource.ID, resource.Version); err != nil {
		return fmt.Errorf("clean stale deps temp roots: %w", err)
	}
	tempRoot, err := os.MkdirTemp(filepath.Dir(storeRoot), "."+resource.ID+"-"+resource.Version+"-*")
	if err != nil {
		return fmt.Errorf("create deps temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	emitPrepareProgress(reporter, PrepareProgress{
		Stage:   "extract",
		Status:  "running",
		Summary: "正在解压 " + managedResourceLabel(resource.Kind),
	}.withResource(&resource, archivePath, storeRoot))
	lastExtractProgress := -1
	lastExtractEntries := 0
	if err := extractWithProgress(ctx, archivePath, resource.ArchiveFormat, tempRoot, extractor, func(progress extractProgress) {
		if progress.Progress == lastExtractProgress {
			if progress.TotalEntries > 0 || progress.ExtractedEntries-lastExtractEntries < 100 {
				return
			}
		}
		lastExtractProgress = progress.Progress
		lastExtractEntries = progress.ExtractedEntries
		emitPrepareProgress(reporter, PrepareProgress{
			Stage:            "extract",
			Status:           "running",
			Progress:         progress.Progress,
			ExtractedEntries: progress.ExtractedEntries,
			TotalEntries:     progress.TotalEntries,
			Summary:          "正在解压 " + managedResourceLabel(resource.Kind),
		}.withResource(&resource, archivePath, storeRoot))
	}); err != nil {
		return fmt.Errorf("extract deps resource %s: %w", resource.Kind, err)
	}
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:    "extract",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceLabel(resource.Kind) + "已解压",
	}.withResource(&resource, archivePath, storeRoot))
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:   "activate",
		Status:  "running",
		Summary: "正在启用 " + managedResourceLabel(resource.Kind),
	}.withResource(&resource, archivePath, storeRoot))
	_ = os.RemoveAll(storeRoot)
	if err := os.Rename(tempRoot, storeRoot); err != nil {
		return fmt.Errorf("activate deps resource %s: %w", resource.Kind, err)
	}
	emitPrepareProgress(reporter, PrepareProgress{
		Stage:    "activate",
		Status:   "succeeded",
		Progress: 100,
		Summary:  managedResourceLabel(resource.Kind) + "已启用",
	}.withResource(&resource, archivePath, storeRoot))
	return nil
}
func removeStaleTempRoots(parent, resourceID, version string) error {
	entries, err := os.ReadDir(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	prefix := "." + resourceID + "-" + version + "-"
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(parent, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
func verifyFileSHA256(path string, want string) error {
	return VerifyFileSHA256(path, want)
}

func acquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	return AcquireLock(ctx, path, now)
}
