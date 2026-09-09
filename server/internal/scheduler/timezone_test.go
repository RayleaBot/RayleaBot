package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestCronTimezoneTransitions(t *testing.T) {
	for _, test := range []struct{ name, zone, cron, after, want string }{
		{"fall back reaches later hour", "America/New_York", "0 2 * * *", "2026-11-01T04:30:00Z", "2026-11-01T07:00:00Z"},
		{"fall back repeats wall time", "America/New_York", "30 1 * * *", "2026-11-01T05:45:00Z", "2026-11-01T06:30:00Z"},
		{"spring gap skips missing time", "America/New_York", "30 2 * * *", "2026-03-08T06:00:00Z", "2026-03-09T06:30:00Z"},
		{"quarter hour timezone", "Asia/Kathmandu", "0 9 * * *", "2026-01-15T00:00:00Z", "2026-01-15T03:15:00Z"},
		{"half hour fall back", "Australia/Lord_Howe", "45 1 * * *", "2026-04-04T14:50:00Z", "2026-04-04T15:15:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			loc, err := time.LoadLocation(test.zone)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := time.Parse(time.RFC3339, test.after)
			got, err := nextCronTime(test.cron, after, loc)
			if err != nil {
				t.Fatal(err)
			}
			if got.Format(time.RFC3339) != test.want {
				t.Fatalf("next = %s, want %s", got.Format(time.RFC3339), test.want)
			}
		})
	}
}

func TestHydrateUsesNewTimezoneForFutureJobsAndRetainsOverdueJobs(t *testing.T) {
	ctx := context.Background()
	repo, err := NewSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	before, err := New(Options{Repository: repo, Logger: logger, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	before.now = func() time.Time { return now }
	job, err := before.Register(ctx, "timezone-plugin", "0 9 * * *", nil)
	if err != nil {
		t.Fatal(err)
	}
	late := job
	late.JobID = "overdue"
	late.NextRun = now.Add(-time.Hour)
	if err := repo.SaveJob(ctx, late); err != nil {
		t.Fatal(err)
	}
	after, err := New(Options{Repository: repo, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	after.now = func() time.Time { return now }
	if after.Timezone() != "Asia/Shanghai" {
		t.Fatalf("default timezone = %q", after.Timezone())
	}
	if err := after.Hydrate(ctx); err != nil {
		t.Fatal(err)
	}
	jobs, err := repo.LoadJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, restored := range jobs {
		want := now.Add(time.Hour)
		if restored.JobID == late.JobID {
			want = late.NextRun
		}
		if !restored.NextRun.Equal(want) {
			t.Errorf("job %s next = %s, want %s", restored.JobID, restored.NextRun, want)
		}
	}
}
