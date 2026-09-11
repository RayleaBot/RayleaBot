package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// WithTx runs fn inside a transaction on db. The transaction is committed when
// fn returns nil and rolled back otherwise, including when fn panics.
func WithTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := ignoreTxDone(tx.Rollback()); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

func ignoreTxDone(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}
