package permission

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

const (
	ListBlacklist = "blacklist"
	ListWhitelist = "whitelist"
)

var ErrGovernanceEntryNotFound = errors.New("governance entry not found")

type Entry struct {
	ID        int64
	Scope     chatevent.IdentityScope
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type EntryRepository interface {
	Contains(context.Context, chatevent.IdentityScope, string, string) (bool, error)
	Get(context.Context, chatevent.IdentityScope, string, string) (Entry, error)
	Add(context.Context, chatevent.IdentityScope, string, string, string) error
	Remove(context.Context, chatevent.IdentityScope, string, string) error
	List(context.Context, string) ([]Entry, error)
}

// SQLiteAccessListRepository uses one storage model for both list policies.
type SQLiteAccessListRepository struct {
	read, write *sqlcgen.Queries
	kind        string
}

func NewSQLiteAccessListRepository(read, write *sql.DB, kind string) *SQLiteAccessListRepository {
	return &SQLiteAccessListRepository{read: sqlcgen.New(read), write: sqlcgen.New(write), kind: kind}
}

func (r *SQLiteAccessListRepository) Contains(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	return r.read.AccessListContains(ctx, sqlcgen.AccessListContainsParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
}

func (r *SQLiteAccessListRepository) Get(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) (Entry, error) {
	entry, err := r.read.AccessListGet(ctx, sqlcgen.AccessListGetParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrGovernanceEntryNotFound
	}
	return fromStoredEntry(entry), err
}

func (r *SQLiteAccessListRepository) Add(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID, reason string) error {
	return r.write.AccessListAdd(ctx, sqlcgen.AccessListAddParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID, Reason: reason, CreatedAt: time.Now().UTC().Format(time.RFC3339)})
}

func (r *SQLiteAccessListRepository) Remove(ctx context.Context, scope chatevent.IdentityScope, entryType, targetID string) error {
	affected, err := r.write.AccessListRemove(ctx, sqlcgen.AccessListRemoveParams{ListKind: r.kind, SourceProtocol: scope.SourceProtocol, SourceAdapter: scope.SourceAdapter, BotID: scope.BotID, EntryType: entryType, TargetID: targetID})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *SQLiteAccessListRepository) List(ctx context.Context, entryType string) ([]Entry, error) {
	stored, err := r.read.AccessListList(ctx, sqlcgen.AccessListListParams{ListKind: r.kind, EntryType: entryType})
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(stored))
	for _, entry := range stored {
		entries = append(entries, fromStoredEntry(entry))
	}
	return entries, nil
}

func fromStoredEntry(entry sqlcgen.AccessListEntry) Entry {
	kind := "instance"
	if entry.SourceAdapter == "" {
		kind = "global"
	}
	return Entry{ID: entry.ID, Scope: chatevent.IdentityScope{Kind: kind, SourceProtocol: entry.SourceProtocol, SourceAdapter: entry.SourceAdapter, BotID: entry.BotID}, EntryType: entry.EntryType, TargetID: entry.TargetID, Reason: entry.Reason, CreatedAt: entry.CreatedAt}
}

type WhitelistStateRepository interface {
	Enabled(context.Context) (bool, error)
	SetEnabled(context.Context, bool) error
}

type SQLiteWhitelistStateRepository struct{ read, write *sqlcgen.Queries }

func NewSQLiteWhitelistStateRepository(read, write *sql.DB) *SQLiteWhitelistStateRepository {
	return &SQLiteWhitelistStateRepository{read: sqlcgen.New(read), write: sqlcgen.New(write)}
}

func (r *SQLiteWhitelistStateRepository) Enabled(ctx context.Context) (bool, error) {
	enabled, err := r.read.WhitelistEnabled(ctx)
	return enabled != 0, err
}

func (r *SQLiteWhitelistStateRepository) SetEnabled(ctx context.Context, enabled bool) error {
	var value int64
	if enabled {
		value = 1
	}
	return r.write.SetWhitelistEnabled(ctx, sqlcgen.SetWhitelistEnabledParams{Enabled: value, UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
}
