package logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

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
