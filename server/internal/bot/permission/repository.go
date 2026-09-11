package permission

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
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

type EntryPage struct {
	Items      []Entry
	Total      int
	EntryCount int
}

type WhitelistStateRepository interface {
	Enabled(context.Context) (bool, error)
	SetEnabled(context.Context, bool) error
}
