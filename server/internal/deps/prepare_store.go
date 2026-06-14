package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	depsprepare "github.com/RayleaBot/RayleaBot/server/internal/deps/prepare"
)

func ensurePreparedResource(
	ctx context.Context,
	repoRoot string,
	resource Resource,
	archivePath string,
	extractor func(context.Context, string, string, string) error,
) error {
	return ensurePreparedResourceWithProgress(ctx, repoRoot, resource, archivePath, extractor, nil)
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
	if err := extractWithProgress(ctx, archivePath, resource.ArchiveFormat, tempRoot, extractor, func(progress extractProgress) {
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
	return depsprepare.VerifyFileSHA256(path, want)
}

func acquireLock(ctx context.Context, path string, now func() time.Time) (func(), error) {
	return depsprepare.AcquireLock(ctx, path, now)
}
