package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

// ResetStoredCredentials owns the offline database operation. Its caller must
// hold the configuration lifecycle lock so no running Manager keeps old state.
func ResetStoredCredentials(ctx context.Context, databasePath string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Stat(databasePath); err != nil {
		return err
	}
	// Resetting credentials must not trigger malformed-database quarantine or
	// create a replacement database as a side effect of opening the store.
	if err := storage.QuickCheckPath(ctx, databasePath); err != nil {
		return err
	}
	store, err := storage.Open(databasePath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	return resetCredentials(ctx, store.Write)
}

func resetCredentials(ctx context.Context, database *sql.DB) error {
	return storage.WithTx(ctx, database, nil, func(tx *sql.Tx) error {
		queries := sqlcgen.New(tx)
		if err := queries.DeleteAllAdminSessions(ctx); err != nil {
			return fmt.Errorf("clear admin sessions: %w", err)
		}
		if err := queries.DeleteBootstrapState(ctx); err != nil {
			return fmt.Errorf("clear bootstrap credentials: %w", err)
		}
		return nil
	})
}
