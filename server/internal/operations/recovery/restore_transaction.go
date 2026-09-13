package recovery

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
)

var errRestoreTargetExists = errors.New("restore destination already exists")

type restoreWrite struct {
	target    string
	staged    string
	installed bool
}

func (w *restoreWorkspace) planWrites(entries map[string]*zip.File, configRelative, databaseRelative string, hasDatabase bool) ([]restoreWrite, error) {
	root, work := w.root, w.work
	writes := make([]restoreWrite, 0, len(entries)+4)
	for name, entry := range entries {
		if name == "backup-manifest.json" || entry.FileInfo().IsDir() {
			continue
		}
		target := name
		if name == "config/user.yaml" {
			target = filepath.ToSlash(configRelative)
		}
		if name == restoredDatabaseEntry && hasDatabase {
			target = databaseRelative
		}
		writes = append(writes, restoreWrite{target: filepath.FromSlash(target), staged: filepath.Join(work, "incoming", filepath.FromSlash(name))})
	}
	if hasDatabase {
		// Sidecars from another database must not attach to the restored one.
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			writes = append(writes, restoreWrite{target: filepath.FromSlash(databaseRelative) + suffix})
		}
	}
	writes = append(writes, restoreWrite{target: filepath.FromSlash(RecoverySummaryPath), staged: filepath.Join(work, "summary.json")})
	sort.Slice(writes, func(i, j int) bool { return writes[i].target < writes[j].target })
	seenTargets := make(map[string]bool, len(writes))
	for _, write := range writes {
		key := strings.ToLower(filepath.ToSlash(write.target))
		if seenTargets[key] {
			return nil, errors.New("restored paths collide after database relocation")
		}
		seenTargets[key] = true
		if err := checkRestoreTarget(root, write.target); err != nil {
			return nil, &RestoreError{Stage: "target", cause: err}
		}
	}
	for target := range seenTargets {
		for parent := path.Dir(target); parent != "."; parent = path.Dir(parent) {
			if seenTargets[parent] {
				return nil, errors.New("restored file paths collide with a parent directory")
			}
		}
	}
	return writes, nil
}

// Lock the destination database even when a different config in the same root
// points at it. The CLI separately owns its configuration lock.
func lockRestoreDatabase(root *os.Root, relative string, present bool) (*filelock.Lock, error) {
	if !present {
		return nil, nil
	}
	lockPath := filepath.FromSlash(relative) + ".lock"
	if err := checkRestorePath(root, lockPath, true); err != nil {
		return nil, err
	}
	if err := root.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	file, err := root.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	return filelock.AcquireFile(file)
}

// Restore only fills targets that do not exist yet, so a failure removes what
// this restore wrote instead of preserving and putting back existing files.
func (w *restoreWorkspace) applyWrites(ctx context.Context, writes []restoreWrite) error {
	var created []string
	for index := range writes {
		if err := w.applyWrite(ctx, &writes[index], &created); err != nil {
			if removeErr := removeRestoreWrites(w.root, writes[:index+1], created, w.deps); removeErr != nil {
				return &RestoreError{Stage: "remove", cause: errors.Join(err, removeErr)}
			}
			return &RestoreError{Stage: "write", cause: err}
		}
	}
	return nil
}

func (w *restoreWorkspace) applyWrite(ctx context.Context, write *restoreWrite, created *[]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkRestoreTarget(w.root, write.target); err != nil {
		return err
	}
	if write.staged == "" {
		return nil
	}
	if err := createRestoreParents(w.root, filepath.Dir(write.target), created); err != nil {
		return err
	}
	if err := w.deps.rename(w.root, write.staged, write.target); err != nil {
		return err
	}
	write.installed = true
	return ctx.Err()
}

func checkRestoreTarget(root *os.Root, name string) error {
	return checkRestorePath(root, name, false)
}

func checkRestorePath(root *os.Root, name string, allowExisting bool) error {
	parts := strings.Split(filepath.ToSlash(name), "/")
	for index := range parts {
		info, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:index+1], "/")))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("restore destination contains a symbolic link")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return errors.New("restore destination parent is not a directory")
		}
		if index == len(parts)-1 {
			if !allowExisting {
				return errRestoreTargetExists
			}
			if !info.Mode().IsRegular() {
				return errors.New("restore destination is not a regular file")
			}
		}
	}
	return nil
}

func createRestoreParents(root *os.Root, directory string, created *[]string) error {
	if directory == "." {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(directory), "/")
	for index := range parts {
		name := filepath.FromSlash(strings.Join(parts[:index+1], "/"))
		if err := root.Mkdir(name, 0o755); err == nil {
			*created = append(*created, name)
		} else if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	return nil
}

func removeRestoreWrites(root *os.Root, writes []restoreWrite, created []string, deps restoreDeps) error {
	var removeErr error
	for index := len(writes) - 1; index >= 0; index-- {
		if writes[index].installed {
			removeErr = errors.Join(removeErr, deps.remove(root, writes[index].target))
		}
	}
	for index := len(created) - 1; index >= 0; index-- {
		if err := root.Remove(created[index]); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr = errors.Join(removeErr, err)
		}
	}
	return removeErr
}
