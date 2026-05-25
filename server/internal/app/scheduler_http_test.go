package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestSystemSchedulerJobListHTTP(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("scheduler.NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.Default(),
		Timezone:   "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}
	job, err := engine.UpsertTaskWithLabel(context.Background(), "weather", "daily_report", "每日早报", "0 8 * * *", []byte(`{"target_type":"group","target_id":"879110321","content":"每日天气推送"}`))
	if err != nil {
		t.Fatalf("UpsertTaskWithLabel: %v", err)
	}
	if err := engine.RecordRunResult(context.Background(), scheduler.RunResult{
		JobID:      job.JobID,
		Outcome:    scheduler.RunOutcomeSuccess,
		Duration:   820 * time.Millisecond,
		OccurredAt: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordRunResult success: %v", err)
	}
	if err := engine.RecordRunResult(context.Background(), scheduler.RunResult{
		JobID:      job.JobID,
		Outcome:    scheduler.RunOutcomeTimeout,
		Duration:   3 * time.Second,
		ErrorCode:  "plugin.event_timeout",
		ErrorText:  "plugin event response timed out",
		OccurredAt: time.Date(2026, 5, 25, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordRunResult timeout: %v", err)
	}

	application := newTestAppState(config.Config{
		Scheduler: config.SchedulerConfig{Timezone: "Asia/Shanghai"},
	}, slog.Default())
	system := newSystemService(systemServiceDeps{
		state:   application.state,
		plugins: plugins.NewCatalog([]plugins.Snapshot{{PluginID: "weather", Name: "天气插件"}}),
	})
	handler := newSystemHTTPHandlers(system, engine).handleSystemSchedulerJobList()
	req := httptest.NewRequest(http.MethodGet, "/api/system/scheduler/jobs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response schedulerJobListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(response.Items))
	}
	item := response.Items[0]
	if item.PluginName != "天气插件" || item.LogLabel != "每日早报" || item.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected item identity: %#v", item)
	}
	if item.Stats.Total != 2 || item.Stats.Success != 1 || item.Stats.Timeout != 1 {
		t.Fatalf("unexpected stats: %#v", item.Stats)
	}
	if item.LastError == nil || item.LastError.Code != "plugin.event_timeout" {
		t.Fatalf("unexpected last error: %#v", item.LastError)
	}
	if item.PayloadSummary.ConversationID != "group:879110321" || item.PayloadSummary.Content != "每日天气推送" {
		t.Fatalf("unexpected payload summary: %#v", item.PayloadSummary)
	}
}

func TestSystemSchedulerJobListHTTPEmpty(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("scheduler.NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.Default(),
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	handler := newSystemHTTPHandlers(nil, engine).handleSystemSchedulerJobList()
	req := httptest.NewRequest(http.MethodGet, "/api/system/scheduler/jobs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response schedulerJobListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(response.Items))
	}
}

func TestSystemSchedulerJobTriggerHTTP(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("scheduler.NewSQLiteRepository: %v", err)
	}

	var mu sync.Mutex
	var fired []string
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.Default(),
		Trigger: func(_ context.Context, job scheduler.Job) {
			mu.Lock()
			defer mu.Unlock()
			fired = append(fired, job.JobID)
		},
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}
	job, err := engine.UpsertTask(context.Background(), "raylea.subscription-hub", "subscription-hub-poll", "*/5 * * * *", nil)
	if err != nil {
		t.Fatalf("UpsertTask: %v", err)
	}

	handler := newSystemHTTPHandlers(nil, engine).handleSystemSchedulerJobTrigger()
	router := chi.NewRouter()
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", handler)
	req := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/jobs/subscription-hub-poll/trigger", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response schedulerJobTriggerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.JobID != "subscription-hub-poll" || response.PluginID != "raylea.subscription-hub" || !response.Triggered {
		t.Fatalf("unexpected response: %#v", response)
	}
	mu.Lock()
	if len(fired) != 1 || fired[0] != "subscription-hub-poll" {
		t.Fatalf("fired = %#v", fired)
	}
	mu.Unlock()
	if jobs := engine.Jobs(); len(jobs) != 1 || !jobs[0].NextRun.Equal(job.NextRun) {
		t.Fatalf("scheduler trigger changed next run: %#v want %v", jobs, job.NextRun)
	}
}

func TestSystemSchedulerJobTriggerHTTPDetachesRequestCancellation(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("scheduler.NewSQLiteRepository: %v", err)
	}

	triggered := make(chan error, 1)
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.Default(),
		Trigger: func(ctx context.Context, _ scheduler.Job) {
			triggered <- ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}
	if _, err := engine.UpsertTask(context.Background(), "weather", "daily_report", "0 8 * * *", nil); err != nil {
		t.Fatalf("UpsertTask: %v", err)
	}

	handler := newSystemHTTPHandlers(nil, engine).handleSystemSchedulerJobTrigger()
	router := chi.NewRouter()
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", handler)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/jobs/daily_report/trigger", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case err := <-triggered:
		if err != nil {
			t.Fatalf("trigger context should not inherit cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler trigger was not called")
	}
}

func TestSystemSchedulerJobTriggerHTTPMissingJob(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("scheduler.NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.Default(),
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	handler := newSystemHTTPHandlers(nil, engine).handleSystemSchedulerJobTrigger()
	router := chi.NewRouter()
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", handler)
	req := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/jobs/missing/trigger", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
