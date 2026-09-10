package pagination

import (
	"net/url"
	"strings"
	"testing"
)

func TestCollectionTraversalRejectsUnboundedOrAmbiguousInputs(t *testing.T) {
	for _, raw := range []string{"limit=0", "limit=101", "limit=-1", "limit=1&limit=2", "cursor=-1", "cursor=01", "cursor=2147483648", "cursor=1&cursor=2", "query=" + strings.Repeat("x", 201)} {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(values); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	query, err := Parse(url.Values{})
	if err != nil || query.Limit != 100 {
		t.Fatalf("default query: %#v %v", query, err)
	}
	items, meta := Slice([]int{1, 2, 3}, Query{Limit: 2})
	if len(items) != 2 || meta.Total != 3 || meta.NextCursor != "2" {
		t.Fatalf("first page: %v %#v", items, meta)
	}
	items, meta = Slice([]int{1, 2, 3}, Query{Limit: 2, Cursor: 2})
	if len(items) != 1 || items[0] != 3 || meta.NextCursor != "" {
		t.Fatalf("last page: %v %#v", items, meta)
	}
	items, meta = Slice([]int{1, 2, 3}, Query{Limit: 2, Cursor: 99})
	if items == nil || len(items) != 0 || meta.Total != 3 || meta.NextCursor != "" {
		t.Fatalf("past end: %v %#v", items, meta)
	}
}
