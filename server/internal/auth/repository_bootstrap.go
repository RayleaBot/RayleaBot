package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) LoadBootstrap(ctx context.Context) (*BootstrapState, error) {
	row, err := r.readQ.LoadBootstrap(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load bootstrap state: %w", err)
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, row.InitializedAt)
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap initialized_at: %w", err)
	}

	return &BootstrapState{
		Identifier:    row.Identifier,
		SecretDigest:  row.SecretDigest,
		SigningKey:    row.SigningKey,
		InitializedAt: parsedTime.UTC(),
	}, nil
}

func (r *SQLiteRepository) SaveBootstrap(ctx context.Context, state BootstrapState, session Claims) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap transaction: %w", err)
	}

	q := r.writeQ.WithTx(tx)

	existing, err := q.CountBootstrap(ctx)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("check bootstrap state: %w", err)
	}
	if existing > 0 {
		_ = tx.Rollback()
		return ErrBootstrapAlreadyInitialized
	}

	if err := q.InsertBootstrap(ctx, sqlcgen.InsertBootstrapParams{
		Identifier:    state.Identifier,
		SecretDigest:  state.SecretDigest,
		SigningKey:    state.SigningKey,
		InitializedAt: state.InitializedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert bootstrap state: %w", err)
	}

	if err := q.UpsertSession(ctx, claimsToUpsertParams(session)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert session %s: %w", session.SessionID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bootstrap transaction: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) UpdateBootstrapSecretDigest(ctx context.Context, secretDigest []byte) error {
	affected, err := r.writeQ.UpdateBootstrapSecretDigest(ctx, secretDigest)
	if err != nil {
		return fmt.Errorf("update bootstrap secret digest: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
