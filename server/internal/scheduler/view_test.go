package scheduler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestViewUsesEngineTimezoneAndTracksDisabledAndRemovedJobs(t *testing.T) {
	t.Parallel()
	repository, err := NewSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Options{Repository: repository, Logger: slog.New(slog.NewTextHandler(discardWriter{}, nil)), Timezone: "America/Los_Angeles"})
	if err != nil {
		t.Fatal(err)
	}
	engine.now = func() time.Time { return time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC) }
	if _, err := engine.UpsertTaskWithLabel(t.Context(), "present", "daily", "Daily", "0 9 * * *", json.RawMessage(`{"target_type":"group","target_id":9007199254740993,"content":"daily"}`)); err != nil {
		t.Fatal(err)
	}
	disabled, err := engine.UpsertTaskWithLabel(t.Context(), "removed", "paused", "", "0 9 * * *", nil)
	if err != nil {
		t.Fatal(err)
	}
	disabled.Enabled = false
	if err := repository.SaveJob(t.Context(), disabled); err != nil {
		t.Fatal(err)
	}
	if err := engine.Hydrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err := NewView(engine, func(id string) string {
		if id == "present" {
			return "Installed plugin"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	items := view.ListJobs().Items
	if len(items) != 2 || items[0].PluginID != "present" || items[1].PluginID != "removed" {
		t.Fatalf("ordered job snapshot = %#v", items)
	}
	first := items[0]
	if first.Timezone != "America/Los_Angeles" || first.NextRun != "2026-07-02T16:00:00Z" || first.PluginName != "Installed plugin" {
		t.Fatalf("engine timezone and metadata drifted: %#v", first)
	}
	if first.PayloadSummary.TargetID != "9007199254740993" || first.PayloadSummary.ConversationID != "group:9007199254740993" {
		t.Fatalf("payload identifier lost precision: %#v", first.PayloadSummary)
	}
	if items[1].Enabled || items[1].PluginName != "removed" {
		t.Fatalf("disabled/missing-plugin view = %#v", items[1])
	}
	if _, err := view.TriggerJob(t.Context(), "paused"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("disabled job was triggered: %v", err)
	}
	if err := engine.UnregisterByPlugin(t.Context(), "present"); err != nil {
		t.Fatal(err)
	}
	items = view.ListJobs().Items
	if len(items) != 1 || items[0].PluginID != "removed" {
		t.Fatalf("removed plugin jobs remained visible: %#v", items)
	}
	if _, err := view.TriggerJob(t.Context(), "daily"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("removed plugin job was triggered: %v", err)
	}
}
