package pluginfile

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

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
