package permission

import (
	"context"
	"errors"
)

const (
	ListBlacklist = "blacklist"
	ListWhitelist = "whitelist"
)

var ErrGovernanceEntryNotFound = errors.New("governance entry not found")

type Entry struct {
	ID        int64
	Scope     Scope
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type EntryRepository interface {
	Contains(context.Context, Scope, string, string) (bool, error)
	Get(context.Context, Scope, string, string) (Entry, error)
	Add(context.Context, Scope, string, string, string) error
	Remove(context.Context, Scope, string, string) error
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
