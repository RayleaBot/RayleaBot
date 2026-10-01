package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/messagestats"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"github.com/go-chi/chi/v5"
)

func TestMessageStatsHTTPMatchesGoldenFixtures(t *testing.T) {
	for _, name := range []string{"ok.message-stats-hourly.yaml", "edge.message-stats-tracking-started.yaml", "edge.message-stats-removed-connection.yaml"} {
		t.Run(name, func(t *testing.T) {
			fixture := testutil.LoadWebAPIFixtureDocument(t, "../fixtures/web-api/"+name)
			expectedJSON, err := json.Marshal(fixture.Response.Body)
			if err != nil {
				t.Fatal(err)
			}
			var expected messagestats.Response
			if err := json.Unmarshal(expectedJSON, &expected); err != nil {
				t.Fatal(err)
			}
			store, err := storage.Open(filepath.Join(t.TempDir(), "stats.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			q := sqlcgen.New(store.Write)
			if err := q.StartMessageTracking(t.Context(), expected.TrackingStartedAt.UnixMilli()); err != nil {
				t.Fatal(err)
			}
			now := expected.TrackingStartedAt
			for _, incident := range expected.Incidents {
				if incident.Kind == "server_stopped" {
					id, err := q.StartMessageStatsRun(t.Context(), sqlcgen.StartMessageStatsRunParams{StartedAtMs: now.UnixMilli(), LastAliveAtMs: incident.StartedAt.UnixMilli()})
					if err != nil {
						t.Fatal(err)
					}
					if err := q.UpdateMessageStatsRun(t.Context(), sqlcgen.UpdateMessageStatsRunParams{ID: id, LastAliveAtMs: incident.StartedAt.UnixMilli(), StoppedAtMs: sql.NullInt64{Int64: incident.StartedAt.UnixMilli(), Valid: true}}); err != nil {
						t.Fatal(err)
					}
					now = *incident.EndedAt
				}
			}
			cfg := config.Config{}
			for _, c := range expected.Connections {
				if c.Configured {
					cfg.Adapters = append(cfg.Adapters, config.AdapterInstance{ID: c.AdapterID, Type: c.Protocol})
				}
			}
			var clock atomic.Int64
			clock.Store(now.UnixMilli())
			service, err := messagestats.New(t.Context(), messagestats.Options{Store: store, Timezone: expected.Timezone, Now: func() time.Time { return time.UnixMilli(clock.Load()) }, CurrentConfig: func() config.Config { return cfg }})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = service.Stop(context.Background()) })
			clock.Store(expected.AsOf.UnixMilli())
			var runID int64
			if err := store.Read.QueryRow("SELECT MAX(id) FROM message_stats_runs").Scan(&runID); err != nil {
				t.Fatal(err)
			}
			for _, c := range expected.Connections {
				meta := sqlcgen.UpsertMessageStatsAdapterParams{AdapterID: c.AdapterID, Protocol: c.Protocol}
				if c.LastReceivedAt != nil {
					meta.LastReceivedAtMs = sql.NullInt64{Int64: c.LastReceivedAt.UnixMilli(), Valid: true}
				}
				if err := q.UpsertMessageStatsAdapter(t.Context(), meta); err != nil {
					t.Fatal(err)
				}
				for i, bucket := range expected.Buckets {
					if err := q.AddMessageStatsHour(t.Context(), sqlcgen.AddMessageStatsHourParams{HourStart: bucket.Unix(), AdapterID: c.AdapterID, Received: c.Received[i], Sent: c.Sent[i]}); err != nil {
						t.Fatal(err)
					}
				}
				if c.Previous != nil {
					at := expected.StartAt.Add(-expected.EndAt.Sub(expected.StartAt))
					if err := q.AddMessageStatsHour(t.Context(), sqlcgen.AddMessageStatsHourParams{HourStart: at.Unix(), AdapterID: c.AdapterID, Received: c.Previous.Received, Sent: c.Previous.Sent}); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, incident := range expected.Incidents {
				if incident.Kind == "adapter_offline" {
					end := sql.NullInt64{}
					if incident.EndedAt != nil {
						end = sql.NullInt64{Int64: incident.EndedAt.UnixMilli(), Valid: true}
					}
					if err := q.UpsertMessageStatsOffline(t.Context(), sqlcgen.UpsertMessageStatsOfflineParams{RunID: runID, AdapterID: incident.AdapterID, StartedAtMs: incident.StartedAt.UnixMilli(), EndedAtMs: end}); err != nil {
						t.Fatal(err)
					}
				}
			}
			router := chi.NewRouter()
			management.NewCoreHandlers(management.CoreDeps{MessageStats: service}).RegisterProtectedRoutes(router)
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(fixture.Request.Method, fixture.Request.Path, nil))
			if result.Code != fixture.Response.Status {
				t.Fatalf("HTTP %d: %s", result.Code, result.Body.String())
			}
			got := testutil.DecodeBody(t, result.Body.Bytes())
			want := testutil.DecodeBody(t, expectedJSON)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("fixture mismatch\ngot=%s\nwant=%s", result.Body.String(), expectedJSON)
			}
			for _, path := range []string{
				"/api/system/message-stats",
				"/api/system/message-stats?start_at=bad&end_at=2026-10-02T00:00:00Z&granularity=hour",
				"/api/system/message-stats?start_at=2026-10-02T00:00:00Z&end_at=2026-10-01T00:00:00Z&granularity=hour",
				"/api/system/message-stats?start_at=2026-10-01T00:00:00Z&end_at=2026-10-01T00:00:00Z&granularity=hour",
				"/api/system/message-stats?start_at=2026-10-01T00:00:00Z&end_at=2026-10-02T00:00:00Z&granularity=week",
				"/api/system/message-stats?start_at=2026-10-01T00:00:00Z&end_at=2026-12-02T00:00:00Z&granularity=hour",
			} {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Code != 400 || testutil.DecodeBody(t, rec.Body.Bytes())["error"].(map[string]any)["code"] != "platform.invalid_request" {
					t.Fatalf("bad validation: %s", rec.Body.String())
				}
			}
		})
	}
}

func TestMessageStatsAppAssemblyAndAuthentication(t *testing.T) {
	application, _, _ := newTestAppWithConfigMutation(t, func(map[string]any) {}, deterministicAuthOptions()...)
	start := time.Now().UTC().Truncate(time.Hour)
	path := "/api/system/message-stats?start_at=" + start.Format(time.RFC3339) + "&end_at=" + start.Add(time.Hour).Format(time.RFC3339) + "&granularity=hour"
	unauthorized := testutil.PerformJSONRequest(t, application, http.MethodGet, path, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated HTTP %d", unauthorized.Code)
	}
	token := issueLoginToken(t, application)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = testutil.TestManagementAuthority
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var result messagestats.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Timezone == "" || result.TrackingStartedAt.IsZero() || len(result.Connections) == 0 || result.Connections[0].Received == nil || result.Incidents == nil {
		t.Fatalf("incomplete assembled view: %+v", result)
	}
}
