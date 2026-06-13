package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
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
