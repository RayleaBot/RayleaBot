package logging

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultSpoolFilename           = "management-logs.spool.jsonl"
	defaultSpoolQuarantineFilename = "management-logs.spool.quarantine.jsonl"
)

type SpoolQueue struct {
	path           string
	quarantinePath string
	mu             sync.Mutex
}

type SpoolFlushResult struct {
	Flushed     int
	Quarantined int
	Pending     int
}

type spoolRecord struct {
	BootID    string         `json:"boot_id,omitempty"`
	LogID     string         `json:"log_id"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Source    string         `json:"source"`
	Message   string         `json:"message"`
	Protocol  string         `json:"protocol,omitempty"`
	PluginID  string         `json:"plugin_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewSpoolQueue(path string) *SpoolQueue {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}

	return &SpoolQueue{
		path:           filepath.Clean(trimmed),
		quarantinePath: filepath.Join(filepath.Dir(trimmed), defaultSpoolQuarantineFilename),
	}
}

func SpoolPathForDatabase(databasePath string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(databasePath)), defaultSpoolFilename)
}

func (q *SpoolQueue) Path() string {
	if q == nil {
		return ""
	}
	return q.path
}

func (q *SpoolQueue) QuarantinePath() string {
	if q == nil {
		return ""
	}
	return q.quarantinePath
}

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

func (q *SpoolQueue) Append(summary Summary) error {
	if q == nil || q.path == "" {
		return errors.New("management log spool path is not configured")
	}

	record := spoolRecordFromSummary(summary)
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode spool record: %w", err)
	}
	payload = append(payload, '\n')

	q.mu.Lock()
	defer q.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(q.path), 0o755); err != nil {
		return fmt.Errorf("prepare spool directory: %w", err)
	}

	file, err := os.OpenFile(q.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open spool file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("append spool record: %w", err)
	}
	return nil
}

func (q *SpoolQueue) Flush(ctx context.Context, repository Repository) (SpoolFlushResult, error) {
	if q == nil || q.path == "" {
		return SpoolFlushResult{}, nil
	}
	if repository == nil {
		return SpoolFlushResult{}, errors.New("management log repository is required")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	lines, err := q.readLines()
	if err != nil {
		return SpoolFlushResult{}, err
	}
	if len(lines) == 0 {
		return SpoolFlushResult{}, nil
	}

	result := SpoolFlushResult{}
	remaining := make([][]byte, 0, len(lines))
	for index, line := range lines {
		select {
		case <-ctx.Done():
			remaining = append(remaining, line)
			remaining = append(remaining, lines[index+1:]...)
			result.Pending = len(remaining)
			if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
				return result, errors.Join(ctx.Err(), rewriteErr)
			}
			return result, ctx.Err()
		default:
		}

		summary, decodeErr := decodeSpoolRecord(line)
		if decodeErr != nil {
			if quarantineErr := q.appendQuarantine(line); quarantineErr != nil {
				remaining = append(remaining, line)
				remaining = append(remaining, lines[index+1:]...)
				result.Pending = len(remaining)
				if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
					return result, errors.Join(decodeErr, quarantineErr, rewriteErr)
				}
				return result, errors.Join(decodeErr, quarantineErr)
			}
			result.Quarantined++
			continue
		}

		if err := repository.SaveSummary(ctx, summary); err != nil {
			remaining = append(remaining, line)
			remaining = append(remaining, lines[index+1:]...)
			result.Pending = len(remaining)
			if rewriteErr := q.rewrite(remaining); rewriteErr != nil {
				return result, errors.Join(err, rewriteErr)
			}
			return result, err
		}

		result.Flushed++
	}

	if err := q.rewrite(remaining); err != nil {
		return result, err
	}
	return result, nil
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

func spoolRecordFromSummary(summary Summary) spoolRecord {
	normalized := NormalizeSummary(summary)
	return spoolRecord{
		BootID:    normalized.BootID,
		LogID:     normalized.LogID,
		Timestamp: normalized.Timestamp,
		Level:     normalized.Level,
		Source:    normalized.Source,
		Message:   normalized.Message,
		Protocol:  normalized.Protocol,
		PluginID:  normalized.PluginID,
		RequestID: normalized.RequestID,
		Details:   cloneDetailsMap(normalized.Details),
	}
}

func decodeSpoolRecord(line []byte) (Summary, error) {
	var record spoolRecord
	if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
		return Summary{}, fmt.Errorf("decode spool record: %w", err)
	}
	return NormalizeSummary(Summary{
		BootID:    record.BootID,
		LogID:     record.LogID,
		Timestamp: record.Timestamp,
		Level:     record.Level,
		Source:    record.Source,
		Message:   record.Message,
		Protocol:  record.Protocol,
		PluginID:  record.PluginID,
		RequestID: record.RequestID,
		Details:   cloneDetailsMap(record.Details),
	}), nil
}
