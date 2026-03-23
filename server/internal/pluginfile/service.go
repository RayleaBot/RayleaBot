package pluginfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidPath   = errors.New("plugin file path is invalid")
	ErrFileTooLarge  = errors.New("plugin file exceeds configured single-file limit")
	ErrQuotaExceeded = errors.New("plugin file workspace exceeds configured total limit")
)

type Limits struct {
	FileMaxBytes  int
	TotalMaxBytes int
}

type ReadResult struct {
	Exists  bool
	Content []byte
	IsText  bool
}

type Service struct {
	root string
}

func NewService(root string) *Service {
	return &Service{root: filepath.Clean(root)}
}

func (s *Service) Read(pluginID, relativePath string) (ReadResult, error) {
	target, info, exists, err := s.resolve(pluginID, relativePath, false)
	if err != nil {
		return ReadResult{}, err
	}
	if !exists {
		return ReadResult{Exists: false}, nil
	}
	if info.IsDir() {
		return ReadResult{}, ErrInvalidPath
	}

	content, err := os.ReadFile(target)
	if err != nil {
		return ReadResult{}, fmt.Errorf("read plugin file: %w", err)
	}
	return ReadResult{
		Exists:  true,
		Content: content,
		IsText:  utf8.Valid(content),
	}, nil
}

func (s *Service) Write(pluginID, relativePath string, content []byte, limits Limits) error {
	if limits.FileMaxBytes > 0 && len(content) > limits.FileMaxBytes {
		return ErrFileTooLarge
	}

	root, err := s.pluginRoot(pluginID)
	if err != nil {
		return err
	}
	target, info, exists, err := s.resolve(pluginID, relativePath, true)
	if err != nil {
		return err
	}
	if exists && info.IsDir() {
		return ErrInvalidPath
	}

	currentSize, err := directorySize(root)
	if err != nil {
		return err
	}

	existingSize := int64(0)
	if exists {
		existingSize = info.Size()
	}
	nextTotal := currentSize - existingSize + int64(len(content))
	if limits.TotalMaxBytes > 0 && nextTotal > int64(limits.TotalMaxBytes) {
		return ErrQuotaExceeded
	}

	parentDir := filepath.Dir(target)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("create plugin file parent directory: %w", err)
	}
	if err := ensureNoSymlinks(root, target); err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("write plugin file: %w", err)
	}
	return nil
}

func (s *Service) Delete(pluginID, relativePath string) (bool, error) {
	target, info, exists, err := s.resolve(pluginID, relativePath, false)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	if info.IsDir() {
		return false, ErrInvalidPath
	}
	if err := os.Remove(target); err != nil {
		return false, fmt.Errorf("delete plugin file: %w", err)
	}
	return true, nil
}

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
