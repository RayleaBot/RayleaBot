package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type quarantineReason struct {
	TimeUTC      string   `json:"time_utc"`
	OriginalPath string   `json:"original_path"`
	Error        string   `json:"error"`
	Files        []string `json:"files"`
}

func quarantineMalformedDatabase(databasePath string, cause error) error {
	databasePath = filepath.Clean(databasePath)
	quarantineDir, err := nextQuarantineDir(databasePath, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return fmt.Errorf("create sqlite quarantine directory: %w", err)
	}

	moved := make([]string, 0, 3)
	for _, path := range databaseSidecarPaths(databasePath) {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat sqlite quarantine source %s: %w", path, err)
		}
		target := filepath.Join(quarantineDir, filepath.Base(path))
		if err := os.Rename(path, target); err != nil {
			return fmt.Errorf("move sqlite database to quarantine: %w", err)
		}
		moved = append(moved, target)
	}
	if len(moved) == 0 {
		return nil
	}

	reason := quarantineReason{
		TimeUTC:      time.Now().UTC().Format(time.RFC3339Nano),
		OriginalPath: databasePath,
		Error:        cause.Error(),
		Files:        moved,
	}
	payload, err := json.MarshalIndent(reason, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sqlite quarantine reason: %w", err)
	}
	if err := os.WriteFile(filepath.Join(quarantineDir, "reason.json"), payload, 0o644); err != nil {
		return fmt.Errorf("write sqlite quarantine reason: %w", err)
	}
	return nil
}

func databaseSidecarPaths(databasePath string) []string {
	return []string{
		databasePath,
		databasePath + "-wal",
		databasePath + "-shm",
	}
}

func nextQuarantineDir(databasePath string, now time.Time) (string, error) {
	parent := filepath.Join(filepath.Dir(databasePath), "quarantine")
	base := "sqlite-malformed-" + now.UTC().Format("20060102T150405Z")
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%02d", base, i)
		}
		path := filepath.Join(parent, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", fmt.Errorf("stat sqlite quarantine directory: %w", err)
		}
	}
	return "", fmt.Errorf("allocate sqlite quarantine directory: too many collisions for %s", base)
}
