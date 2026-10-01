package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestCanceledRunsPreserveLastFailureInMemoryAndStorage(t *testing.T) {
	for _, withFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "no failure", true: "previous failure"}[withFailure], func(t *testing.T) {
			repo, err := NewSQLiteRepository(openTestStore(t))
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			job := Job{JobID: "fixture", PluginID: "fixture", CronExpr: "0 8 * * *", Enabled: true, CreatedAt: now, UpdatedAt: now, NextRun: now.Add(time.Hour)}
			if err := repo.SaveJob(t.Context(), job); err != nil {
				t.Fatal(err)
			}
			engine := &Engine{repo: repo, jobs: map[string]Job{job.JobID: job}, now: time.Now}
			if withFailure {
				if err := engine.RecordRunResult(t.Context(), RunResult{JobID: job.JobID, Outcome: RunOutcomeFailed, ErrorCode: "plugin.internal_error", ErrorText: "fixture failure", OccurredAt: now}); err != nil {
					t.Fatal(err)
				}
			}
			run := RunResult{JobID: job.JobID, Outcome: RunOutcomeOther, ErrorCode: errorcodes.PluginEventCanceled, OccurredAt: now.Add(time.Minute), Duration: 50 * time.Millisecond}
			if err := engine.RecordRunResult(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			stored, err := repo.LoadJobs(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []Job{engine.jobs[job.JobID], stored[0]} {
				if got.RunStats.Other != 1 || got.LastRun == nil || !got.LastRun.Equal(run.OccurredAt) || got.LastDurationMS != 50 {
					t.Fatalf("run accounting = %#v", got)
				}
				if withFailure {
					if got.LastError == nil || got.LastError.Code != "plugin.internal_error" || !got.LastError.At.Equal(now) {
						t.Fatalf("failure overwritten: %#v", got.LastError)
					}
				} else if got.LastError != nil {
					t.Fatalf("cancellation became last_error: %#v", got.LastError)
				}
			}
		})
	}
}
