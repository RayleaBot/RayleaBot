package recovery

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
)

type restoreWorkspace struct {
	root           *os.Root
	repoRoot, work string
	deps           restoreDeps
	keepWork       bool
}

func newRestoreWorkspace(repoRoot string, deps restoreDeps) (*restoreWorkspace, error) {
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(repoRoot); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("restore root must be a real directory")
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	work := ".restore-" + rand.Text()
	if err := root.Mkdir(work, 0o700); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &restoreWorkspace{root: root, repoRoot: repoRoot, work: work, deps: deps}, nil
}

func (w *restoreWorkspace) close(committed bool, cause error) error {
	if !w.keepWork {
		if err := w.deps.removeAll(w.root, w.work); err != nil {
			stage := "preflight cleanup"
			if committed {
				stage = "cleanup"
			}
			cause = &RestoreError{Stage: stage, RecoveryDirectory: filepath.Join(w.repoRoot, w.work), cause: errors.Join(cause, err)}
		}
	}
	return errors.Join(cause, w.root.Close())
}

func (w *restoreWorkspace) extract(ctx context.Context, entries map[string]*zip.File) error {
	root, work, deps := w.root, w.work, w.deps
	for name, entry := range entries {
		if name == "backup-manifest.json" || entry.FileInfo().IsDir() {
			continue
		}
		if err := root.MkdirAll(filepath.Join(work, "incoming", filepath.Dir(filepath.FromSlash(name))), 0o700); err != nil {
			return err
		}
		file, openErr := root.OpenFile(filepath.Join(work, "incoming", filepath.FromSlash(name)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm())
		if openErr != nil {
			return openErr
		}
		copyErr := deps.copy(ctx, entry, file)
		copyErr = errors.Join(copyErr, file.Close())
		if copyErr != nil {
			return &RestoreError{Stage: "extract", cause: copyErr}
		}
	}
	return nil
}
