package filestore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) pluginRoot(pluginID string) (string, error) {
	trimmedID := strings.TrimSpace(pluginID)
	if trimmedID == "" {
		return "", ErrInvalidPath
	}
	root := filepath.Join(s.root, trimmedID)
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve plugin file root: %w", err)
	}
	return absolute, nil
}

func (s *Service) resolve(pluginID, relativePath string, allowMissingLeaf bool) (string, fs.FileInfo, bool, error) {
	root, err := s.pluginRoot(pluginID)
	if err != nil {
		return "", nil, false, err
	}
	normalizedPath, err := normalizeRelativePath(relativePath, false)
	if err != nil {
		return "", nil, false, err
	}
	target := filepath.Join(root, normalizedPath)
	if err := ensureWithinRoot(root, target); err != nil {
		return "", nil, false, err
	}
	if err := ensureNoSymlinks(root, target); err != nil {
		return "", nil, false, err
	}

	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		if allowMissingLeaf {
			return target, nil, false, nil
		}
		return target, nil, false, nil
	}
	if err != nil {
		return "", nil, false, fmt.Errorf("stat plugin file path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", nil, false, ErrInvalidPath
	}
	return target, info, true, nil
}

func normalizeRelativePath(raw string, allowEmpty bool) (string, error) {
	if raw == "" {
		if allowEmpty {
			return "", nil
		}
		return "", ErrInvalidPath
	}

	normalized := filepath.Clean(filepath.FromSlash(raw))
	if normalized == "." {
		if allowEmpty {
			return "", nil
		}
		return "", ErrInvalidPath
	}
	if filepath.IsAbs(normalized) || filepath.VolumeName(normalized) != "" {
		return "", ErrInvalidPath
	}
	if normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return normalized, nil
}

func ensureWithinRoot(root, target string) error {
	relativePath, err := filepath.Rel(root, target)
	if err != nil {
		return ErrInvalidPath
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return ErrInvalidPath
	}
	return nil
}

func ensureNoSymlinks(root, target string) error {
	relativePath, err := filepath.Rel(root, target)
	if err != nil {
		return ErrInvalidPath
	}
	if relativePath == "." {
		return nil
	}

	current := root
	for _, part := range strings.Split(relativePath, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("stat plugin file path component: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidPath
		}
	}
	return nil
}
