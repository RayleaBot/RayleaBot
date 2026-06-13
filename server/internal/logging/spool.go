package logging

import (
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
