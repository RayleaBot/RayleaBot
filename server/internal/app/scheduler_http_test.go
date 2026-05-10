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

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

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
