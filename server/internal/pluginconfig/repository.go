package pluginconfig

import (
	"context"
	"database/sql"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Repository interface {
	SeedDefaults(ctx context.Context, pluginID string, values map[string]any) (bool, error)
	Read(ctx context.Context, pluginID string, keys []string) (map[string]any, error)
	ReadAll(ctx context.Context, pluginID string) (map[string]any, error)
	Write(ctx context.Context, pluginID string, values map[string]any) ([]string, error)
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	read   *sql.DB
	write  *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		read:   store.Read,
		write:  store.Write,
	}, nil
}
