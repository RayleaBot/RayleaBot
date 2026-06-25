package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

type AppliedMigration struct {
	Version   int
	Name      string
	AppliedAt string
}

func (s *Store) ListAppliedMigrations(ctx context.Context) ([]AppliedMigration, error) {
	if s == nil || s.Read == nil {
		return nil, errors.New("sqlite store is required")
	}
	rows, err := sqlcgen.New(s.Read).ListSchemaMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schema migrations: %w", err)
	}
	items := make([]AppliedMigration, 0, len(rows))
	for _, row := range rows {
		items = append(items, AppliedMigration{
			Version:   int(row.Version),
			Name:      row.Name,
			AppliedAt: row.AppliedAt,
		})
	}
	return items, nil
}
