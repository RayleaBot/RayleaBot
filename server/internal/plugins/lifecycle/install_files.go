package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func waitForInstallRenameRetry(ctx context.Context) error {
	timer := time.NewTimer(installRenameRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *InstallService) renameInstallPath(ctx context.Context, source, target string) error {
	if installRenameAttempts < 1 {
		return errors.New("install rename attempts must be positive")
	}
	var lastErr error
	for attempt := 0; attempt < installRenameAttempts; attempt++ {
		if contextErr := ctx.Err(); contextErr != nil {
			if lastErr != nil {
				return errors.Join(lastErr, contextErr)
			}
			return contextErr
		}
		lastErr = s.deps.rename(source, target)
		if lastErr == nil {
			return nil
		}
		if !s.deps.retryRename(lastErr) || attempt+1 == installRenameAttempts {
			return lastErr
		}
		if waitErr := s.deps.waitRename(ctx); waitErr != nil {
			return errors.Join(lastErr, waitErr)
		}
	}
	return errors.New("install rename attempts exhausted")
}

func copyDirectory(ctx context.Context, sourceRoot, targetRoot string) error {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", sourceRoot)
	}
	if err := os.MkdirAll(targetRoot, info.Mode().Perm()); err != nil {
		return err
	}

	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceRoot {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink entries are not supported in install sources")
		}

		relativePath, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relativePath)

		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(sourcePath, targetPath string) (err error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func(release func() error) { _ = release() }(sourceFile.Close)

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, targetFile.Close()) }()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return nil
}
