package market

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginartifact "github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func storeSnapshotFixture(t *testing.T) Catalog {
	t.Helper()
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	var document Catalog
	if err := json.Unmarshal(catalogJSON(platform), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func storeSnapshotPayload(t *testing.T, document Catalog) []byte {
	t.Helper()
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestStorePaginationPreservesStableOrderingAndSearch(t *testing.T) {
	t.Parallel()
	document := storeSnapshotFixture(t)
	base := document.Entries[0]
	document.Entries = nil
	for index, id := range []string{"first", "second", "third", "fourth"} {
		entry := cloneEntry(base)
		entry.ID, entry.Name, entry.Recommended = id, "same", index == 1
		entry.Keywords = []string{"needle"}
		switch index {
		case 1:
			entry.Name = "SAME"
			entry.CurrentRelease.PublishedAt = "2026-09-03T08:00:00+08:00"
		case 2:
			entry.Name, entry.Keywords = "alpha", []string{"different"}
			entry.CurrentRelease.PublishedAt = "2026-09-04T00:00:00Z"
		case 3:
			entry.CurrentRelease = nil
		}
		document.Entries = append(document.Entries, entry)
	}
	payload := storeSnapshotPayload(t, document)
	service := newTestService(t, emptyCatalog{}, nil, newMemoryRepository(payload), staticCatalogTransport(payload))
	for _, test := range []struct {
		name  string
		query Query
		ids   []string
		total int
		next  string
	}{
		{"name", Query{Sort: "name", Limit: 4}, []string{"third", "first", "second", "fourth"}, 4, ""},
		{"updated_equal_instants", Query{Sort: "updated", Limit: 4}, []string{"third", "first", "second", "fourth"}, 4, ""},
		{"recommended_page", Query{Sort: "recommended", Limit: 2}, []string{"second", "third"}, 4, "2"},
		{"deep_page", Query{Sort: "name", Cursor: 2, Limit: 2}, []string{"second", "fourth"}, 4, ""},
		{"filtered_page", Query{Sort: "updated", Text: " NEEDLE ", Cursor: 1, Limit: 1}, []string{"second"}, 3, "2"},
		{"past_end", Query{Sort: "name", Cursor: 99, Limit: 2}, nil, 4, ""},
		{"no_match", Query{Text: "absent-token"}, nil, 0, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := service.List(test.query)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, item := range result.Items {
				ids = append(ids, item.ID)
			}
			if !reflect.DeepEqual(ids, test.ids) || result.Total != test.total || result.NextCursor != test.next {
				t.Fatalf("page = %+v, ids=%v", result, ids)
			}
		})
	}
}

func TestStorePublishedSnapshotAndResponsesOwnMutableFields(t *testing.T) {
	t.Parallel()
	document := storeSnapshotFixture(t)
	wantKeyword := document.Entries[0].Keywords[0]
	wantAsset := document.Entries[0].CurrentRelease.Assets[0]
	snapshot := newCatalogSnapshot(Source{ID: OfficialSourceID}, document, time.Now())
	document.Entries[0].Keywords[0] = "caller changed input"
	document.Entries[0].CurrentRelease.Assets[0].URL = "https://changed.invalid/fixture.zip"
	if snapshot.catalog.Entries[0].Keywords[0] != wantKeyword || snapshot.entriesByID["raylea.echo"].CurrentRelease.Assets[0] != wantAsset {
		t.Fatal("published snapshot borrows mutable source fields")
	}
	payload := storeSnapshotPayload(t, snapshot.catalog)
	installer := &stubInstaller{}
	service := newTestService(t, emptyCatalog{}, installer, newMemoryRepository(payload), staticCatalogTransport(payload))
	list, err := service.List(Query{})
	if err != nil {
		t.Fatal(err)
	}
	list.Items[0].Keywords[0] = "changed list"
	list.Items[0].LatestRelease.Version = "99.0.0"
	*list.Source.RefreshedAt = time.Time{}
	detail, ok := service.Get(OfficialSourceID, "raylea.echo")
	if !ok || detail.Plugin.Keywords[0] != wantKeyword || detail.Plugin.LatestRelease.Version != "0.4.0" || detail.Source.RefreshedAt.IsZero() {
		t.Fatalf("list mutation escaped: %+v", detail)
	}
	detail.Plugin.Keywords[0] = "changed detail"
	detail.Plugin.LatestRelease.Version = "88.0.0"
	if detail.CurrentRelease.Version != "0.4.0" {
		t.Fatal("detail release views share mutable storage")
	}
	detail.CurrentRelease.Version = "77.0.0"
	*detail.Source.RefreshedAt = time.Time{}
	if _, err := service.Install(t.Context(), InstallRequest{SourceID: OfficialSourceID, PluginID: "raylea.echo", TrustedCodeConfirmed: true}); err != nil {
		t.Fatal(err)
	}
	if installer.accepted.ResolvedSource != wantAsset.URL || installer.accepted.ExpectedArchiveSHA256 != wantAsset.ArchiveSHA256 || installer.accepted.ExpectedVersion != "0.4.0" {
		t.Fatalf("response mutation changed installation source: %+v", installer.accepted)
	}
	again, _ := service.Get(OfficialSourceID, "raylea.echo")
	if again.Plugin.Keywords[0] != wantKeyword || again.CurrentRelease.Version != "0.4.0" || again.Source.RefreshedAt.IsZero() {
		t.Fatal("detail mutation escaped into source snapshot")
	}
}

func TestStoreReadProjectionDoesNotCacheInstallationConfirmation(t *testing.T) {
	t.Parallel()
	document := storeSnapshotFixture(t)
	payload := storeSnapshotPayload(t, document)
	installed := plugincatalog.New([]plugins.Snapshot{{PluginID: "raylea.echo", Version: "0.3.0", PackageSourceType: "catalog", PackageSourceRef: OfficialSourceID}})
	installer := &stubInstaller{}
	service := newTestService(t, installed, installer, newMemoryRepository(payload), staticCatalogTransport(payload))
	page, err := service.List(Query{})
	if err != nil || len(page.Items[0].ConfirmationReasons) != 0 {
		t.Fatalf("initial source projection: %+v %v", page, err)
	}
	installed.Replace([]plugins.Snapshot{{PluginID: "raylea.echo", Version: "0.2.0", PackageSourceType: "local_directory"}})
	if _, err := service.Install(t.Context(), InstallRequest{SourceID: OfficialSourceID, PluginID: "raylea.echo"}); !errors.Is(err, plugins.ErrTrustedCodeConfirmation) {
		t.Fatalf("installation reused stale source confirmation: %v", err)
	}
	detail, ok := service.Get(OfficialSourceID, "raylea.echo")
	if !ok || detail.Plugin.InstalledVersion != "0.2.0" || !reflect.DeepEqual(detail.Plugin.ConfirmationReasons, []string{"source_changed"}) {
		t.Fatalf("detail retained old installed state: %+v", detail)
	}
	installed.Replace(nil)
	page, err = service.List(Query{})
	if err != nil || page.Items[0].InstalledVersion != "" || !reflect.DeepEqual(page.Items[0].ConfirmationReasons, []string{"first_install"}) {
		t.Fatalf("uninstalled entry remained installed: %+v %v", page, err)
	}
}

func TestStoreConcurrentRefreshPublishesWholeReadSnapshots(t *testing.T) {
	base := storeSnapshotFixture(t).Entries[0]
	payloads := make([][]byte, 2)
	for generation := range payloads {
		document := Catalog{CatalogVersion: "2"}
		for index := 0; index <= generation; index++ {
			entry := cloneEntry(base)
			entry.ID = "fixture.first"
			if index != 0 {
				entry.ID = "fixture.second"
			}
			entry.Name = []string{"old", "new"}[generation]
			entry.Keywords = []string{entry.Name}
			document.Entries = append(document.Entries, entry)
		}
		payloads[generation] = storeSnapshotPayload(t, document)
	}
	var current atomic.Value
	current.Store(payloads[0])
	service := newTestService(t, emptyCatalog{}, nil, newMemoryRepository(payloads[0]), roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(current.Load().([]byte)))}, nil
	}))
	var workers sync.WaitGroup
	workers.Go(func() {
		for index := range 30 {
			current.Store(payloads[index%2])
			if _, err := service.Refresh(context.Background(), OfficialSourceID); err != nil {
				t.Error(err)
				return
			}
		}
	})
	for range 2 {
		workers.Go(func() {
			for range 100 {
				page, err := service.List(Query{Sort: "updated", Limit: 10})
				if err != nil || len(page.Items) != page.Total || page.Source.EntryCount != page.Total || page.Total < 1 || page.Total > 2 {
					t.Errorf("partial source publication: %+v %v", page, err)
					return
				}
				want := []string{"old", "new"}[page.Total-1]
				for _, entry := range page.Items {
					if entry.Name != want || entry.Keywords[0] != want {
						t.Errorf("mixed source generations: %+v", page)
						return
					}
					entry.Keywords[0] = "caller-owned mutation"
				}
				*page.Source.RefreshedAt = time.Time{}
				detail, ok := service.Get(OfficialSourceID, "fixture.first")
				if !ok || detail.Plugin.Keywords[0] != detail.Plugin.Name || detail.Source.RefreshedAt.IsZero() {
					t.Errorf("detail snapshot changed: %+v", detail)
					return
				}
			}
		})
	}
	workers.Wait()
}
