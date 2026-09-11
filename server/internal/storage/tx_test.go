package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestWithTxCommitsWhenCallbackSucceeds(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()

	err := WithTx(ctx, store.Write, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO secret_store (key, value, created_at, updated_at) VALUES ('k', 'v', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		return err
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	var count int
	if err := store.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM secret_store WHERE key = 'k'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestWithTxRollsBackWhenCallbackFails(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	ctx := context.Background()
	sentinel := errors.New("boom")

	err := WithTx(ctx, store.Write, nil, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO secret_store (key, value, created_at, updated_at) VALUES ('k', 'v', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	var count int
	if err := store.Read.QueryRowContext(ctx, `SELECT COUNT(*) FROM secret_store WHERE key = 'k'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
