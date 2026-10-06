package storage

import (
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"testing"
)

func TestAllConnectionsAndReplacementsKeepPragmas(t *testing.T) {
	store := mustOpenStore(t, filepath.Join(t.TempDir(), "state # +.db"))
	t.Cleanup(func() { _ = store.Close() })
	check := func(conn *sql.Conn, readOnly int) {
		t.Helper()
		for name, want := range map[string]int{
			"foreign_keys": 1, "busy_timeout": int(defaultBusyTimeout.Milliseconds()),
			"synchronous": 2, "wal_autocheckpoint": defaultWALAutoCheckpointPage, "query_only": readOnly,
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
