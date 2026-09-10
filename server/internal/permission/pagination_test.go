package permission

import (
	"fmt"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
)

func TestAccessListPagesTraverseAllRowsAndFilterBeforeLimit(t *testing.T) {
	store := openPermissionTestStore(t)
	repo := NewSQLiteAccessListRepository(store.Read, store.Write, ListWhitelist)
	scope := chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}
	for index := range 205 {
		kind, reason := "user", "ordinary"
		if index%2 == 0 {
			kind, reason = "group", "50% MATCH"
		}
		if err := repo.Add(t.Context(), scope, kind, fmt.Sprintf("entry-%03d", index), reason); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int64]bool{}
	for cursor := 0; cursor < 205; cursor += 100 {
		page, err := repo.Page(t.Context(), pagination.Query{Limit: 100, Cursor: cursor}, "")
		if err != nil || page.Total != 205 || page.EntryCount != 205 || len(page.Items) > 100 {
			t.Fatalf("page=%#v err=%v", page, err)
		}
		for _, entry := range page.Items {
			if seen[entry.ID] {
				t.Fatalf("duplicate row %d", entry.ID)
			}
			seen[entry.ID] = true
		}
	}
	if len(seen) != 205 {
		t.Fatalf("traversed %d rows", len(seen))
	}
	page, err := repo.Page(t.Context(), pagination.Query{Limit: 2, Text: "% match"}, "group")
	if err != nil || page.Total != 103 || page.EntryCount != 205 || len(page.Items) != 2 {
		t.Fatalf("filtered page=%#v err=%v", page, err)
	}
	for _, entry := range page.Items {
		if entry.EntryType != "group" {
			t.Fatalf("filter missed: %#v", entry)
		}
	}
}
