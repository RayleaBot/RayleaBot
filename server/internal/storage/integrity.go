package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IntegrityError struct {
	Path    string
	Results []string
}

func (e *IntegrityError) Error() string {
	if e == nil {
		return "sqlite integrity check failed"
	}
	if len(e.Results) == 0 {
		return "sqlite integrity check failed: " + e.Path
	}
	return fmt.Sprintf("sqlite integrity check failed: %s: %s", e.Path, strings.Join(e.Results, "; "))
}

type quarantineReason struct {
	TimeUTC      string   `json:"time_utc"`
	OriginalPath string   `json:"original_path"`
	Error        string   `json:"error"`
	Files        []string `json:"files"`
}

func QuickCheckPath(ctx context.Context, path string) error {
	path = filepath.Clean(path)
	db, err := sql.Open(sqliteDriverName, sqliteReadOnlyDSN(path))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
		return err
	}
	return QuickCheck(ctx, db, path)
}

func sqliteReadOnlyDSN(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = path
	}
	slashPath := filepath.ToSlash(absolutePath)
	if volume := filepath.VolumeName(slashPath); volume != "" && !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	uri := url.URL{Scheme: "file", Path: slashPath}
	query := uri.Query()
	query.Set("mode", "ro")
	uri.RawQuery = query.Encode()
	return uri.String()
}

func QuickCheck(ctx context.Context, db *sql.DB, path string) error {
	if db == nil {
		return errors.New("sqlite database handle is required")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer rows.Close()

	results := make([]string, 0, 1)
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(results) == 1 && strings.EqualFold(strings.TrimSpace(results[0]), "ok") {
		return nil
	}
	return &IntegrityError{Path: path, Results: results}
}

func isSQLiteCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	var integrityErr *IntegrityError
	if errors.As(err, &integrityErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database disk image is malformed") ||
		strings.Contains(message, "file is not a database") ||
		strings.Contains(message, "database corruption") ||
		strings.Contains(message, "malformed")
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
