package sqlite

import (
	"errors"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
)

func TestSaveSummariesIsAtomicAndRetriesIdempotently(t *testing.T) {
	repository, store := openLoggingRepositoryStore(t)
	summaries := []logging.Summary{
		{LogID: "first", Timestamp: "2026-10-07T00:00:00.000000001Z", Level: "info", Message: "first"},
		{LogID: "second", Timestamp: "2026-10-07T00:00:00.000000002Z", Level: "warn", Message: "second"},
	}
	if _, err := store.Write.Exec(`CREATE TRIGGER reject_second BEFORE INSERT ON management_logs WHEN NEW.log_id='second' BEGIN SELECT RAISE(ABORT,'fixture failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveSummaries(t.Context(), summaries); err == nil {
		t.Fatal("failing batch succeeded")
	}
	if _, err := repository.GetSummary(t.Context(), "first"); !errors.Is(err, logging.ErrLogNotFound) {
		t.Fatalf("partial batch persisted: %v", err)
	}
	if _, err := store.Write.Exec(`DROP TRIGGER reject_second`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := repository.SaveSummaries(t.Context(), summaries); err != nil {
			t.Fatal(err)
		}
	}
	items, err := repository.ListSummaries(t.Context(), logging.Query{})
	if err != nil || len(items) != 2 || items[0].LogID != "first" || items[1].LogID != "second" {
		t.Fatalf("batch replay: %+v, %v", items, err)
	}
}
