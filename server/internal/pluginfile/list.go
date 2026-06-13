package pluginfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Service) List(pluginID, prefix string) ([]string, error) {
	root, err := s.pluginRoot(pluginID)
	if err != nil {
		return nil, err
	}
	normalizedPrefix, err := normalizeRelativePath(prefix, true)
	if err != nil {
		return nil, err
	}
	if normalizedPrefix != "" {
		prefixTarget := filepath.Join(root, normalizedPrefix)
		if err := ensureNoSymlinks(root, prefixTarget); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat plugin file root: %w", err)
	}

	normalizedPrefix = filepath.ToSlash(normalizedPrefix)
	items := make([]string, 0)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if normalizedPrefix == "" || strings.HasPrefix(relativePath, normalizedPrefix) {
			items = append(items, relativePath)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("list plugin files: %w", walkErr)
	}
	sort.Strings(items)
	return items, nil
}

func directorySize(root string) (int64, error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("stat plugin file root: %w", err)
	}

	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("calculate plugin file directory size: %w", err)
	}
	return total, nil
}
