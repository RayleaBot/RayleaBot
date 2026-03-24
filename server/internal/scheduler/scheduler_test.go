package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"rayleabot/server/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSQLiteRepository_SaveAndLoad(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 3, 22, 12, 30, 0, 0, time.UTC)

	job := Job{
		JobID:     "sched_abc123",
		PluginID:  "hello-python",
		CronExpr:  "*/30 * * * *",
		Payload:   json.RawMessage(`{"key":"value"}`),
		Enabled:   true,
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.SaveJob(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	loaded, err := repo.LoadJobs(ctx)
	if err != nil {
		t.Fatalf("load jobs: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d jobs, want 1", len(loaded))
	}

	got := loaded[0]
	if got.JobID != job.JobID {
		t.Errorf("JobID = %q, want %q", got.JobID, job.JobID)
	}
	if got.PluginID != job.PluginID {
		t.Errorf("PluginID = %q, want %q", got.PluginID, job.PluginID)
	}
	if got.CronExpr != job.CronExpr {
		t.Errorf("CronExpr = %q, want %q", got.CronExpr, job.CronExpr)
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true")
	}
	if !got.NextRun.Equal(nextRun) {
		t.Errorf("NextRun = %v, want %v", got.NextRun, nextRun)
	}
}

func TestSQLiteRepository_Delete(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	job := Job{
		JobID:     "sched_del1",
		PluginID:  "test-plugin",
		CronExpr:  "0 * * * *",
		Payload:   json.RawMessage("{}"),
		Enabled:   true,
		NextRun:   now.Add(time.Hour),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.SaveJob(ctx, job); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.DeleteJob(ctx, "sched_del1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	loaded, err := repo.LoadJobs(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded %d jobs, want 0", len(loaded))
	}
}

func TestSQLiteRepository_DeleteByPlugin(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	for i, pid := range []string{"plugin-a", "plugin-a", "plugin-b"} {
		job := Job{
			JobID:     "sched_bp" + string(rune('0'+i)),
			PluginID:  pid,
			CronExpr:  "0 * * * *",
			Payload:   json.RawMessage("{}"),
			Enabled:   true,
			NextRun:   now.Add(time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repo.SaveJob(ctx, job); err != nil {
			t.Fatalf("save job %d: %v", i, err)
		}
	}

	if err := repo.DeleteJobsByPlugin(ctx, "plugin-a"); err != nil {
		t.Fatalf("delete by plugin: %v", err)
	}

	loaded, err := repo.LoadJobs(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d jobs, want 1", len(loaded))
	}
	if loaded[0].PluginID != "plugin-b" {
		t.Errorf("remaining job plugin = %q, want plugin-b", loaded[0].PluginID)
	}
}

func TestSQLiteRepository_NilStore(t *testing.T) {
	t.Parallel()
	_, err := NewSQLiteRepository(nil)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestCron_NextTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expr   string
		after  time.Time
		expect time.Time
	}{
		{
			name:   "every 30 minutes",
			expr:   "*/30 * * * *",
			after:  time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
			expect: time.Date(2026, 3, 22, 12, 30, 0, 0, time.UTC),
		},
		{
			name:   "top of every hour",
			expr:   "0 * * * *",
			after:  time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC),
			expect: time.Date(2026, 3, 22, 13, 0, 0, 0, time.UTC),
		},
		{
			name:   "specific time daily",
			expr:   "30 8 * * *",
			after:  time.Date(2026, 3, 22, 9, 0, 0, 0, time.UTC),
			expect: time.Date(2026, 3, 23, 8, 30, 0, 0, time.UTC),
		},
		{
			name:   "every 5 minutes",
			expr:   "*/5 * * * *",
			after:  time.Date(2026, 3, 22, 12, 3, 0, 0, time.UTC),
			expect: time.Date(2026, 3, 22, 12, 5, 0, 0, time.UTC),
		},
		{
			name:   "weekday only (Mon-Fri)",
			expr:   "0 9 * * 1-5",
			after:  time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), // Sunday
			expect: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC),  // Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := nextCronTime(tt.expr, tt.after, time.UTC)
			if err != nil {
				t.Fatalf("nextCronTime(%q): %v", tt.expr, err)
			}
			if !got.Equal(tt.expect) {
				t.Errorf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestCron_InvalidExpr(t *testing.T) {
	t.Parallel()
	_, err := nextCronTime("bad", time.Now(), time.UTC)
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestEngine_RegisterAndHydrate(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	logger := testLogger()
	engine, err := New(Options{
		Repository: repo,
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx := context.Background()
	job, err := engine.Register(ctx, "test-plugin", "*/15 * * * *", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if job.JobID == "" {
		t.Fatal("job ID is empty")
	}
	if job.PluginID != "test-plugin" {
		t.Errorf("PluginID = %q, want test-plugin", job.PluginID)
	}

	// Create a new engine and hydrate to verify persistence.
	engine2, err := New(Options{
		Repository: repo,
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("new engine2: %v", err)
	}
	if err := engine2.Hydrate(ctx); err != nil {
		t.Fatalf("hydrate: %v", err)
	}

	jobs := engine2.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	if jobs[0].JobID != job.JobID {
		t.Errorf("hydrated job ID = %q, want %q", jobs[0].JobID, job.JobID)
	}
}

func TestEngine_Unregister(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	engine, err := New(Options{
		Repository: repo,
		Logger:     testLogger(),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx := context.Background()
	job, err := engine.Register(ctx, "test-plugin", "0 * * * *", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := engine.Unregister(ctx, job.JobID); err != nil {
		t.Fatalf("unregister: %v", err)
	}

	if len(engine.Jobs()) != 0 {
		t.Fatal("expected 0 jobs after unregister")
	}
}

func TestEngine_UpsertTask(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	engine, err := New(Options{
		Repository: repo,
		Logger:     testLogger(),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx := context.Background()
	first, err := engine.UpsertTask(ctx, "weather", "daily_report", "0 8 * * *", json.RawMessage(`{"topic":"daily_report"}`))
	if err != nil {
		t.Fatalf("first UpsertTask: %v", err)
	}
	second, err := engine.UpsertTask(ctx, "weather", "daily_report", "30 9 * * *", json.RawMessage(`{"topic":"daily_report_v2"}`))
	if err != nil {
		t.Fatalf("second UpsertTask: %v", err)
	}
	if second.JobID != "daily_report" {
		t.Fatalf("JobID = %q, want daily_report", second.JobID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("CreatedAt changed across upsert: %v vs %v", second.CreatedAt, first.CreatedAt)
	}

	jobs := engine.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].CronExpr != "30 9 * * *" {
		t.Fatalf("CronExpr = %q, want 30 9 * * *", jobs[0].CronExpr)
	}
}

func TestEngine_TickFiresDueJob(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	repo, err := NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	var mu sync.Mutex
	var fired []string

	engine, err := New(Options{
		Repository: repo,
		Logger:     testLogger(),
		Trigger: func(_ context.Context, job Job) {
			mu.Lock()
			fired = append(fired, job.JobID)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Fix the clock to a known time.
	baseTime := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return baseTime }

	ctx := context.Background()
	job, err := engine.Register(ctx, "test-plugin", "*/30 * * * *", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Advance clock past the next_run.
	engine.now = func() time.Time { return job.NextRun.Add(time.Minute) }
	engine.tick()

	// Allow async persist goroutine to complete.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(fired) != 1 {
		t.Fatalf("fired %d jobs, want 1", len(fired))
	}
	if fired[0] != job.JobID {
		t.Errorf("fired job = %q, want %q", fired[0], job.JobID)
	}
}

func testLogger() *slog.Logger {
	return slog.Default()
}
