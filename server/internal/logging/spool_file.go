package logging

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (q *SpoolQueue) HasEntries() bool {
	if q == nil || q.path == "" {
		return false
	}

	info, err := os.Stat(q.path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func (q *SpoolQueue) readLines() ([][]byte, error) {
	file, err := os.Open(q.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open spool file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	lines := make([][]byte, 0)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lines = append(lines, append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan spool file: %w", err)
	}
	return lines, nil
}

func (q *SpoolQueue) rewrite(lines [][]byte) error {
	if len(lines) == 0 {
		if err := os.Remove(q.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove empty spool file: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(q.path), 0o755); err != nil {
		return fmt.Errorf("prepare spool directory: %w", err)
	}

	var buffer bytes.Buffer
	for _, line := range lines {
		buffer.Write(bytes.TrimSpace(line))
		buffer.WriteByte('\n')
	}

	tempPath := q.path + ".tmp"
	if err := os.WriteFile(tempPath, buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("rewrite spool temp file: %w", err)
	}
	if err := os.Rename(tempPath, q.path); err != nil {
		return fmt.Errorf("replace spool file: %w", err)
	}
	return nil
}

func (q *SpoolQueue) appendQuarantine(line []byte) error {
	if q == nil || q.quarantinePath == "" {
		return errors.New("management log quarantine path is not configured")
	}

	if err := os.MkdirAll(filepath.Dir(q.quarantinePath), 0o755); err != nil {
		return fmt.Errorf("prepare quarantine directory: %w", err)
	}

	file, err := os.OpenFile(q.quarantinePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open quarantine file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(bytes.TrimSpace(line), '\n')); err != nil {
		return fmt.Errorf("append quarantine line: %w", err)
	}
	return nil
}
