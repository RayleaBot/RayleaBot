package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
)

func openWithProtection(path string, lock *filelock.Lock) (*Store, error) {
	if databaseFileExists(path) {
		if err := QuickCheckPath(context.Background(), path); err != nil {
			if !isSQLiteCorruptionError(err) {
				return nil, fmt.Errorf("check sqlite integrity: %w", err)
			}
			if err := quarantineMalformedDatabase(path, err); err != nil {
				return nil, err
			}
		}
	}

	store, err := openConfigured(path, lock)
	if err == nil {
		return store, nil
	}
	if databaseFileExists(path) && isSQLiteCorruptionError(err) {
		if quarantineErr := quarantineMalformedDatabase(path, err); quarantineErr != nil {
			return nil, quarantineErr
		}
		return openConfigured(path, lock)
	}
	return nil, err
}

func openConfigured(path string, lock *filelock.Lock) (*Store, error) {
	writeDSN, err := configuredDSN(path, false)
	if err != nil {
		return nil, err
	}
	readDSN, err := configuredDSN(path, true)
	if err != nil {
		return nil, err
	}
	// Validate and migrate before enabling persistent WAL mode. A refused schema
	// must leave the original database unchanged.
	writeDB, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite write handle: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	readDB, err := sql.Open(sqliteDriverName, readDSN)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("open sqlite read handle: %w", err)
	}
	readDB.SetMaxOpenConns(defaultReadMaxConns)
	readDB.SetMaxIdleConns(defaultReadMaxConns)

	cleanup := func(cause error) (*Store, error) {
		_ = readDB.Close()
		_ = writeDB.Close()
		return nil, cause
	}

	if err := initializeSchema(context.Background(), writeDB); err != nil {
		return cleanup(fmt.Errorf("initialize sqlite schema: %w", err))
	}
	if err := writeDB.Close(); err != nil {
		return cleanup(fmt.Errorf("close sqlite schema handle: %w", err))
	}
	configuredWriteDB, err := sql.Open(sqliteDriverName, writeDSN)
	if err != nil {
		return cleanup(fmt.Errorf("open configured sqlite write handle: %w", err))
	}
	writeDB = configuredWriteDB
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)
	if err := writeDB.PingContext(context.Background()); err != nil {
		return cleanup(fmt.Errorf("configure sqlite write handle: %w", err))
	}
	if err := readDB.PingContext(context.Background()); err != nil {
		return cleanup(fmt.Errorf("configure sqlite read handle: %w", err))
	}

	return &Store{
		Path:  path,
		Read:  readDB,
		Write: writeDB,
		lock:  lock,
	}, nil
}

func configuredDSN(path string, readOnly bool) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve sqlite path: %w", err)
	}
	uriPath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	// DSN options are applied to every connection, including replacements after cancellation.
	query := url.Values{
		"_busy_timeout": {strconv.FormatInt(defaultBusyTimeout.Milliseconds(), 10)},
		"_foreign_keys": {"on"},
		"_journal_mode": {"wal"},
		"_synchronous":  {"full"},
		"_pragma":       {fmt.Sprintf("wal_autocheckpoint=%d", defaultWALAutoCheckpointPage)},
	}
	if readOnly {
		query.Set("_query_only", "on")
	} else {
		query.Set("_txlock", "immediate")
	}
	dsn := url.URL{Scheme: "file", Path: uriPath, RawQuery: query.Encode()}
	return dsn.String(), nil
}

func databaseLockPath(databasePath string) string {
	return databasePath + ".lock"
}

func databaseFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
