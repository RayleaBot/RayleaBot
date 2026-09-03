package pluginmarket

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{readQ: sqlcgen.New(store.Read), writeQ: sqlcgen.New(store.Write)}, nil
}

func (r *SQLiteRepository) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := r.readQ.ListPluginStoreSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plugin store sources: %w", err)
	}
	items := make([]Source, 0, len(rows))
	for _, row := range rows {
		items = append(items, Source{ID: row.SourceID, Name: row.Name, URL: row.Url, Official: row.Official == 1})
	}
	return items, nil
}

func (r *SQLiteRepository) CreateSource(ctx context.Context, source Source) error {
	if err := r.writeQ.CreatePluginStoreSource(ctx, sqlcgen.CreatePluginStoreSourceParams{
		SourceID: source.ID,
		Name:     source.Name,
		Url:      source.URL,
	}); err != nil {
		return fmt.Errorf("create plugin store source: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) UpdateSource(ctx context.Context, source Source) error {
	updated, err := r.writeQ.UpdatePluginStoreSource(ctx, sqlcgen.UpdatePluginStoreSourceParams{
		SourceID: source.ID,
		Name:     source.Name,
		Url:      source.URL,
	})
	if err != nil {
		return fmt.Errorf("update plugin store source: %w", err)
	}
	if updated == 0 {
		return ErrSourceNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteSource(ctx context.Context, sourceID string) error {
	deleted, err := r.writeQ.DeletePluginStoreSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("delete plugin store source: %w", err)
	}
	if deleted == 0 {
		return ErrSourceNotFound
	}
	return nil
}

func (r *SQLiteRepository) LoadCatalogs(ctx context.Context) ([]CachedCatalog, error) {
	rows, err := r.readQ.LoadPluginStoreCatalogCaches(ctx)
	if err != nil {
		return nil, fmt.Errorf("load plugin store catalog caches: %w", err)
	}
	items := make([]CachedCatalog, 0, len(rows))
	for _, row := range rows {
		refreshedAt, err := time.Parse(time.RFC3339Nano, row.RefreshedAt)
		if err != nil {
			return nil, fmt.Errorf("parse plugin store cache time for %s: %w", row.SourceID, err)
		}
		items = append(items, CachedCatalog{SourceID: row.SourceID, Payload: []byte(row.CatalogJson), RefreshedAt: refreshedAt})
	}
	return items, nil
}

func (r *SQLiteRepository) SaveCatalog(ctx context.Context, cached CachedCatalog) error {
	if err := r.writeQ.SavePluginStoreCatalogCache(ctx, sqlcgen.SavePluginStoreCatalogCacheParams{
		SourceID:    cached.SourceID,
		CatalogJson: string(cached.Payload),
		RefreshedAt: cached.RefreshedAt.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("save plugin store catalog cache: %w", err)
	}
	return nil
}
