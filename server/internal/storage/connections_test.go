package storage

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// A deferred transaction would take the write lock at its first write and could then fail with SQLITE_BUSY
// halfway through; write transactions take it when they begin.
func TestWriteTransactionsTakeTheWriteLockAtBegin(t *testing.T) {
	store := mustOpenStore(t, filepath.Join(t.TempDir(), "state.db"))
	t.Cleanup(func() { _ = store.Close() })
	contender, err := sql.Open(sqliteDriverName, store.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = contender.Close() })

	tx, err := store.Write.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = contender.ExecContext(t.Context(), "BEGIN IMMEDIATE")
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_BUSY {
		t.Fatalf("competing writer error = %v, want SQLITE_BUSY", err)
	}
}

func TestAllConnectionsAndReplacementsKeepPragmas(t *testing.T) {
	store := mustOpenStore(t, filepath.Join(t.TempDir(), "state # +.db"))
	t.Cleanup(func() { _ = store.Close() })
	check := func(conn *sql.Conn, readOnly int) {
		t.Helper()
		for name, want := range map[string]int{
			"foreign_keys": 1, "busy_timeout": int(defaultBusyTimeout.Milliseconds()),
			"synchronous": 1 + readOnly, "wal_autocheckpoint": defaultWALAutoCheckpointPage, "query_only": readOnly,
		} {
			var got int
			if err := conn.QueryRowContext(t.Context(), "PRAGMA "+name).Scan(&got); err != nil || got != want {
				t.Fatalf("%s = %d, want %d: %v", name, got, want, err)
			}
		}
		if readOnly == 1 {
			if _, err := conn.ExecContext(t.Context(), "CREATE TABLE forbidden_write (id INTEGER)"); err == nil {
				t.Fatal("read connection allowed a write")
			}
		}
	}
	var held []*sql.Conn
	for range defaultReadMaxConns {
		conn, err := store.Read.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, conn)
		t.Cleanup(func() { _ = conn.Close() })
		check(conn, 1)
	}
	for _, conn := range held {
		_ = conn.Close()
	}
	for _, pool := range []*sql.DB{store.Read, store.Write} {
		conn, err := pool.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
		replacement, err := pool.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		readOnly := 0
		if pool == store.Read {
			readOnly = 1
		}
		check(replacement, readOnly)
		_ = replacement.Close()
	}
}
