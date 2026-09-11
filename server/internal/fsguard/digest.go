package fsguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// SHA256File hashes a bounded regular file. Callers own symlink/reparse policy.
func SHA256File(ctx context.Context, path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("digest input is not a regular file: %s", path)
	}
	if limit >= 0 && info.Size() > limit {
		return "", ErrSizeLimit
	}
	return SHA256(ctx, file, limit)
}

// SHA256Directory hashes sorted relative file names and contents, each followed
// by a NUL separator. Empty directories do not contribute to the digest.
func SHA256Directory(ctx context.Context, root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", root)
	}
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("digest input is not a regular file: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(files)
	digest := sha256.New()
	for _, relative := range files {
		_, _ = io.WriteString(digest, filepath.ToSlash(relative)+"\x00")
		file, err := os.Open(filepath.Join(root, relative))
		if err != nil {
			return "", err
		}
		_, copyErr := CopyAtMost(ctx, digest, file, math.MaxInt64)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = digest.Write([]byte{0})
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
