// Package permissiontest provides shared list behavior for permission test doubles.
package permissiontest

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

func Page(ctx context.Context, list func(context.Context, string) ([]permission.Entry, error), query pagination.Query, entryType string) (permission.EntryPage, error) {
	users, err := list(ctx, "user")
	if err != nil {
		return permission.EntryPage{}, err
	}
	groups, err := list(ctx, "group")
	if err != nil {
		return permission.EntryPage{}, err
	}
	all := append(users, groups...)
	filtered := make([]permission.Entry, 0, len(all))
	for _, item := range all {
		if (entryType == "" || item.EntryType == entryType) && pagination.Matches(query.Text, item.TargetID, item.Reason) {
			filtered = append(filtered, item)
		}
	}
	items, meta := pagination.Slice(filtered, query)
	return permission.EntryPage{Items: items, Total: meta.Total, EntryCount: len(all)}, nil
}
