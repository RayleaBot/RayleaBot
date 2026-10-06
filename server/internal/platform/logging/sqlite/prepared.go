package sqlite

import (
	"context"
	"database/sql"
	"sync"
)

// preparedLogWrites reuses the two fixed sqlc writes. The store owns the
// connection lifetime and releases their driver statements when it closes.
type preparedLogWrites struct {
	*sql.DB
	mu         sync.RWMutex
	statements map[string]*sql.Stmt
}

func (w *preparedLogWrites) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	w.mu.RLock()
	statement := w.statements[query]
	full := len(w.statements) == 2
	w.mu.RUnlock()
	if statement != nil {
		return statement.ExecContext(ctx, args...)
	}
	if full {
		return w.DB.ExecContext(ctx, query, args...)
	}

	// Preparing can wait for the single write connection. Do not hold the
	// cache lock while waiting, so other callers retain their cancellation budget.
	prepared, err := w.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	statement = w.statements[query]
	if statement == nil && len(w.statements) < 2 {
		w.statements[query] = prepared
		statement = prepared
	}
	w.mu.Unlock()
	if statement == prepared {
		return statement.ExecContext(ctx, args...)
	}
	if statement != nil {
		_ = prepared.Close()
		return statement.ExecContext(ctx, args...)
	}
	defer func() { _ = prepared.Close() }()
	return prepared.ExecContext(ctx, args...)
}
