package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

// AccessListRepository uses one storage model for both list policies.
type AccessListRepository struct {
	read, write *sqlcgen.Queries
	readDB      *sql.DB
	kind        string
}

func NewAccessListRepository(read, write *sql.DB, kind string) *AccessListRepository {
	return &AccessListRepository{read: sqlcgen.New(read), write: sqlcgen.New(write), readDB: read, kind: kind}
}

func (r *AccessListRepository) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	tx, err := r.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return permission.EntryPage{}, err
	}
	defer func() { _ = tx.Rollback() }()
	queries := r.read.WithTx(tx)
	countArgs := sqlcgen.AccessListCountParams{ListKind: r.kind, EntryType: entryType, SearchText: query.Text}
	total, err := queries.AccessListCount(ctx, countArgs)
	if err != nil {
		return permission.EntryPage{}, err
	}
	entryCount := total
	if entryType != "" || query.Text != "" {
		entryCount, err = queries.AccessListCount(ctx, sqlcgen.AccessListCountParams{ListKind: r.kind, EntryType: "", SearchText: ""})
		if err != nil {
			return permission.EntryPage{}, err
		}
	}
	rows, err := queries.AccessListPage(ctx, sqlcgen.AccessListPageParams{ListKind: r.kind, EntryType: entryType, SearchText: query.Text, PageLimit: int64(query.Limit), PageOffset: int64(query.Cursor)})
	if err != nil {
		return permission.EntryPage{}, err
	}
	items := make([]permission.Entry, 0, len(rows))
	for _, entry := range rows {
		items = append(items, fromStoredEntry(entry))
	}
	if err := tx.Commit(); err != nil {
		return permission.EntryPage{}, err
	}
	return permission.EntryPage{Items: items, Total: int(total), EntryCount: int(entryCount)}, nil
}

func (r *AccessListRepository) Contains(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	return r.read.AccessListContains(ctx, sqlcgen.AccessListContainsParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
}

func (r *AccessListRepository) Get(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	entry, err := r.read.AccessListGet(ctx, sqlcgen.AccessListGetParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
	if errors.Is(err, sql.ErrNoRows) {
		return permission.Entry{}, permission.ErrGovernanceEntryNotFound
	}
	return fromStoredEntry(entry), err
}

func (r *AccessListRepository) Add(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) error {
	return r.write.AccessListAdd(ctx, sqlcgen.AccessListAddParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID, Reason: reason, CreatedAt: time.Now().UTC().Format(time.RFC3339)})
}

func (r *AccessListRepository) Remove(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	affected, err := r.write.AccessListRemove(ctx, sqlcgen.AccessListRemoveParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return permission.ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *AccessListRepository) List(ctx context.Context, entryType string) ([]permission.Entry, error) {
	stored, err := r.read.AccessListList(ctx, sqlcgen.AccessListListParams{ListKind: r.kind, EntryType: entryType})
	if err != nil {
		return nil, err
	}
	entries := make([]permission.Entry, 0, len(stored))
	for _, entry := range stored {
		entries = append(entries, fromStoredEntry(entry))
	}
	return entries, nil
}

func fromStoredEntry(entry sqlcgen.AccessListEntry) permission.Entry {
	kind := "instance"
	if entry.SourceAdapter == "" {
		kind = "global"
	}
	return permission.Entry{ID: entry.ID, Scope: chatevent.IdentityScope{Kind: kind, SourceProtocol: entry.SourceProtocol, SourceAdapter: entry.SourceAdapter, BotID: entry.BotID}, EntryType: entry.EntryType, TargetID: entry.TargetID, Reason: entry.Reason, CreatedAt: entry.CreatedAt}
}

type WhitelistStateRepository struct{ read, write *sqlcgen.Queries }

func NewWhitelistStateRepository(read, write *sql.DB) *WhitelistStateRepository {
	return &WhitelistStateRepository{read: sqlcgen.New(read), write: sqlcgen.New(write)}
}

func (r *WhitelistStateRepository) Enabled(ctx context.Context) (bool, error) {
	enabled, err := r.read.WhitelistEnabled(ctx)
	return enabled != 0, err
}

func (r *WhitelistStateRepository) SetEnabled(ctx context.Context, enabled bool) error {
	var value int64
	if enabled {
		value = 1
	}
	return r.write.SetWhitelistEnabled(ctx, sqlcgen.SetWhitelistEnabledParams{Enabled: value, UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
}
