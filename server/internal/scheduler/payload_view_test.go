package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

func TestJobPayloadViewDoesNotSharePublicCloneState(t *testing.T) {
	engine, view, _ := newPayloadViewTestService(t, func(_ context.Context, job Job) {
		copy(job.Payload, []byte(`{"content":"firing"}`))
	})
	payload := json.RawMessage(`{"content":"source"}`)
	created, err := engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "* * * * *", payload)
	if err != nil {
		t.Fatal(err)
	}
	copy(payload, []byte(`{"content":"inputs"}`))
	copy(created.Payload, []byte(`{"content":"caller"}`))
	if got := view.jobSummary(created).PayloadSummary.Content; got != "caller" {
		t.Fatalf("new caller copy reused engine summary: %q", got)
	}
	triggered, err := engine.Trigger(t.Context(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if got := view.jobSummary(triggered).PayloadSummary.Content; got != "firing" {
		t.Fatalf("trigger copy reused engine summary: %q", got)
	}
	assertPayloadViewContent(t, view, "source")
	copyAfterRead := engine.Jobs()[0]
	copy(copyAfterRead.Payload, []byte(`{"content":"second"}`))
	if got := view.jobSummary(copyAfterRead).PayloadSummary.Content; got != "second" {
		t.Fatalf("caller copy reused a populated engine summary: %q", got)
	}
	assertPayloadViewContent(t, view, "source")
}

func TestJobPayloadViewReplacesRestoresAndDeletesWithJob(t *testing.T) {
	engine, view, repository := newPayloadViewTestService(t, nil)
	upsert := func(content string) Job {
		t.Helper()
		job, err := engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "* * * * *", json.RawMessage(fmt.Sprintf(`{"content":%q}`, content)))
		if err != nil {
			t.Fatal(err)
		}
		return job
	}
	upsert("old payload")
	oldSnapshot := engine.viewJobs()[0]
	current := upsert("new payload")
	assertPayloadViewContent(t, view, "new payload")
	if got := oldSnapshot.payloadSummary().Content; got != "old payload" {
		t.Fatalf("replacement changed a prior unread snapshot: %q", got)
	}
	if page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 10, Text: "old payload"}}); page.Total != 0 {
		t.Fatal("replaced payload remains searchable")
	}
	if err := engine.RecordRunResult(t.Context(), RunResult{
		JobID: current.JobID, Revision: current.Revision, Outcome: RunOutcomeSuccess,
		OccurredAt: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC), Duration: time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	assertPayloadViewContent(t, view, "new payload")
	if got := view.ListJobs().Items[0]; got.Stats.Success != 1 || got.LastDurationMS != 1000 {
		t.Fatalf("run result was not preserved: %+v", got)
	}
	engine.now = func() time.Time { return current.NextRun }
	engine.tick()
	assertPayloadViewContent(t, view, "new payload")
	if !engine.Jobs()[0].NextRun.After(current.NextRun) {
		t.Fatal("tick did not advance the published job")
	}
	if deleted, err := engine.DeletePluginTask(t.Context(), "plugin", "task"); err != nil || !deleted {
		t.Fatalf("delete: %v %v", deleted, err)
	}
	if got := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 10}}); got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("deleted job remains visible: %+v", got)
	}
	upsert("restored payload")
	assertPayloadViewContent(t, view, "restored payload")
	stored := engine.Jobs()[0]
	stored.Payload = json.RawMessage(`{"content":"loaded from repository"}`)
	if err := repository.SaveJob(t.Context(), stored); err != nil {
		t.Fatal(err)
	}
	if err := engine.Hydrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertPayloadViewContent(t, view, "loaded from repository")
	loaded, err := repository.LoadJobs(t.Context())
	if err != nil || len(loaded) != 1 {
		t.Fatalf("load repository: %v, %v", loaded, err)
	}
	loaded[0].Payload = json.RawMessage(`{"content":"repository caller"}`)
	if got := view.jobSummary(loaded[0]).PayloadSummary.Content; got != "repository caller" {
		t.Fatalf("repository caller reused engine summary: %q", got)
	}
	assertPayloadViewContent(t, view, "loaded from repository")
	if err := engine.UnregisterByPlugin(t.Context(), "plugin"); err != nil {
		t.Fatal(err)
	}
	if got := view.ListJobs(); got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("unregistered job remains visible: %+v", got)
	}
}

func TestJobPayloadViewPreservesJSONSummarySemantics(t *testing.T) {
	engine, view, _ := newPayloadViewTestService(t, nil)
	for _, tc := range []struct {
		name, raw string
		want      JobPayloadSummary
	}{
		{"empty", "", JobPayloadSummary{}},
		{"null", " null ", JobPayloadSummary{}},
		{"array", `[{"content":"ignored"}]`, JobPayloadSummary{}},
		{"scalar", `"ignored"`, JobPayloadSummary{}},
		{"malformed", `{"content":"ignored","extra":`, JobPayloadSummary{}},
		{"malformed-unknown", `{"content":"ignored","extra":[false,]}`, JobPayloadSummary{}},
		{"additional-json", `{"content":"first"} {"content":"second"}`, JobPayloadSummary{Content: "first"}},
		{"additional-garbage", `{"content":"first"} invalid`, JobPayloadSummary{Content: "first"}},
		{"duplicate-key", `{"content":"old","content":null,"summary":"fallback"}`, JobPayloadSummary{Content: "fallback"}},
		{"case-sensitive-key", `{"CONTENT":"ignored","summary":"fallback"}`, JobPayloadSummary{Content: "fallback"}},
		{"fallback", `{"target_type":" ","type":"group","target_id":{},"group_id":null,"user_id":false,"conversation_id":" ","session_id":" session ","content":[],"summary":" ","title":{},"topic":" topic ","message":"later"}`, JobPayloadSummary{ConversationID: "session", TargetType: "group", TargetID: "false", Content: "topic"}},
		{"numbers", `{"target_type":"group","target_id":9007199254740993,"content":1e100000}`, JobPayloadSummary{ConversationID: "group:9007199254740993", TargetType: "group", TargetID: "9007199254740993", Content: "1e100000"}},
		{"negative-zero", `{"target_id":-0,"content":true}`, JobPayloadSummary{TargetID: "-0", Content: "true"}},
		{"invalid-utf8", "{\"content\":\"a\xff\xfe\"}", JobPayloadSummary{Content: "a\ufffd\ufffd"}},
		{"surrogate", `{"content":"\ud800"}`, JobPayloadSummary{Content: "\ufffd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "* * * * *", json.RawMessage(tc.raw)); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1}})
				if len(page.Items) != 1 || page.Items[0].PayloadSummary != tc.want {
					t.Fatalf("page summary = %+v, want %+v", page.Items, tc.want)
				}
				list := view.ListJobs()
				if len(list.Items) != 1 || list.Items[0].PayloadSummary != tc.want {
					t.Fatalf("list summary = %+v, want %+v", list.Items, tc.want)
				}
			}
		})
	}
}

func TestJobPayloadViewConcurrentReadsAndReplacements(t *testing.T) {
	engine, view, _ := newPayloadViewTestService(t, nil)
	upsert := func(index int) (Job, error) {
		return engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "* * * * *", json.RawMessage(fmt.Sprintf(`{"target_id":%d,"content":"payload-%d"}`, index, index)))
	}
	if _, err := upsert(0); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errors := make(chan error, 9)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Go(func() {
			<-start
			for range 64 {
				page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1, Text: "payload"}})
				if len(page.Items) > 1 {
					errors <- fmt.Errorf("multiple jobs for one identity: %+v", page)
					return
				}
				if len(page.Items) == 1 {
					summary := page.Items[0].PayloadSummary
					if summary.Content != "payload-"+summary.TargetID {
						errors <- fmt.Errorf("mixed payload generations: %+v", summary)
						return
					}
				}
			}
		})
	}
	workers.Go(func() {
		<-start
		for index := 1; index <= 24; index++ {
			if index%8 == 0 {
				if _, err := engine.DeletePluginTask(t.Context(), "plugin", "task"); err != nil {
					errors <- err
					return
				}
			}
			job, err := upsert(index)
			if err != nil {
				errors <- err
				return
			}
			if err := engine.RecordRunResult(t.Context(), RunResult{JobID: job.JobID, Revision: job.Revision, Outcome: RunOutcomeSuccess}); err != nil {
				errors <- err
				return
			}
		}
	})
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	assertPayloadViewContent(t, view, "payload-24")
}

func TestJobPayloadViewLargeValuesAndReplacements(t *testing.T) {
	engine, view, _ := newPayloadViewTestService(t, nil)
	for _, tc := range []struct {
		name, content, target, extra string
	}{
		{"large content", strings.Repeat("large content ", 8192), "target", ""},
		{"large target", "brief", strings.Repeat("target", 2048), ""},
		{"large unused value", "brief", "target", strings.Repeat("unused", 32768)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for generation := 0; generation < 2; generation++ {
				content := fmt.Sprintf("generation-%d %s", generation, tc.content)
				payload, err := json.Marshal(map[string]any{
					"content": content, "target_type": "group", "target_id": tc.target, "extra": tc.extra,
				})
				if err != nil {
					t.Fatal(err)
				}
				created, err := engine.UpsertTaskWithLabel(t.Context(), "plugin", "task", "", "* * * * *", payload)
				if err != nil {
					t.Fatal(err)
				}
				created.Payload = json.RawMessage(`{"content":"caller"}`)
				if got := view.jobSummary(created).PayloadSummary.Content; got != "caller" {
					t.Fatalf("caller summary = %q", got)
				}
				start := make(chan struct{})
				var readers sync.WaitGroup
				for range 8 {
					readers.Go(func() {
						<-start
						for range 2 {
							page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1, Text: fmt.Sprintf("generation-%d", generation)}})
							if len(page.Items) != 1 {
								t.Errorf("large payload generation %d is missing", generation)
								return
							}
							got := page.Items[0].PayloadSummary
							if got.Content != strings.TrimSpace(content) || got.TargetID != tc.target || got.ConversationID != "group:"+tc.target {
								t.Errorf("large payload generation %d was truncated or replaced", generation)
								return
							}
						}
					})
				}
				close(start)
				readers.Wait()
				if generation > 0 {
					if page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 1, Text: "generation-0"}}); page.Total != 0 {
						t.Fatal("large previous payload is still searchable")
					}
				}
			}
		})
	}
}

func newPayloadViewTestService(t *testing.T, trigger TriggerFunc) (*Engine, *View, *SQLiteRepository) {
	t.Helper()
	repository, err := NewSQLiteRepository(openTestStore(t))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Options{Repository: repository, Logger: slog.New(slog.NewTextHandler(discardWriter{}, nil)), Timezone: "UTC", Trigger: trigger})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Stop)
	view, err := NewView(engine, nil)
	if err != nil {
		t.Fatal(err)
	}
	return engine, view, repository
}

func assertPayloadViewContent(t *testing.T, view *View, content string) {
	t.Helper()
	page := view.ListJobsPage(JobQuery{Query: pagination.Query{Limit: 10, Text: content}})
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].PayloadSummary.Content != content {
		t.Fatalf("payload page = %+v, want content %q", page, content)
	}
}
