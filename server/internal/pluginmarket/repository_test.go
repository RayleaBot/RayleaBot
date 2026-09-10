package pluginmarket

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSourceRepositoryClassifiesUniqueURLWithoutMatchingErrorText(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	first := Source{ID: "first", Name: "first", URL: "https://example.invalid/first.json"}
	second := Source{ID: "second", Name: "second", URL: "https://example.invalid/second.json"}
	for _, item := range []Source{first, second} {
		if err := repository.CreateSource(t.Context(), item); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.CreateSource(t.Context(), Source{ID: "third", Name: "third", URL: first.URL}); !errors.Is(err, ErrSourceConflict) {
		t.Fatalf("duplicate URL: %v", err)
	}
	second.URL = first.URL
	if err := repository.UpdateSource(t.Context(), second); !errors.Is(err, ErrSourceConflict) {
		t.Fatalf("conflicting update: %v", err)
	}
	if isSourceURLConflict(errors.New("UNIQUE constraint failed")) {
		t.Fatal("plain upstream text was treated as a database constraint")
	}
}
