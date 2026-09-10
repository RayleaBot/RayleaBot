package pluginmarket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginartifact "github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

func TestStoreDetailExposesOnlyTheCurrentRelease(t *testing.T) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	for _, published := range []bool{true, false} {
		payload := catalogJSON(platform, "0.4.0", "0.4.0")
		if !published {
			var catalog Catalog
			if err := json.Unmarshal(payload, &catalog); err != nil {
				t.Fatal(err)
			}
			catalog.Entries[0].CurrentRelease = nil
			payload, err = json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
		}
		service := newTestService(t, emptyCatalog{}, nil, newMemoryRepository(payload), staticCatalogTransport(payload))
		detail, ok := service.Get(OfficialSourceID, "raylea.echo")
		if !ok || (detail.CurrentRelease != nil) != published {
			t.Fatalf("published=%v: unexpected current release: %#v", published, detail)
		}
		encoded, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(encoded, []byte(`"releases"`)) || !published && !bytes.Contains(encoded, []byte(`"current_release":null`)) {
			t.Fatalf("unexpected release collection: %s", encoded)
		}
	}
}

func TestServiceLoadsCachedCatalogAndKeepsItAfterRefreshFailure(t *testing.T) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	payload := catalogJSON(platform, "0.4.0", "0.4.0")
	repository := newMemoryRepository(payload)
	service := newTestService(t, emptyCatalog{}, nil, repository, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	}))

	result, err := service.List(Query{SourceID: OfficialSourceID})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].InstallState != "available" || !result.Source.Cached {
		t.Fatalf("unexpected cached result: %#v", result)
	}
	if _, err := service.Refresh(context.Background(), OfficialSourceID); ErrorCode(err) != CodeCatalogUnavailable {
		t.Fatalf("Refresh() error = %v", err)
	}
	result, err = service.List(Query{SourceID: OfficialSourceID})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("failed refresh replaced cached catalog: result=%#v err=%v", result, err)
	}
}

func TestServiceCustomSourceLifecycle(t *testing.T) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	payload := catalogJSON(platform, "0.4.0", "0.4.0")
	repository := newMemoryRepository(nil)
	service := newTestService(t, emptyCatalog{}, nil, repository, staticCatalogTransport(payload))

	created, err := service.CreateSource(context.Background(), SourceInput{Name: "社区源", URL: "https://plugins.example/catalog.json"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Official || !created.Cached || created.EntryCount != 1 {
		t.Fatalf("created source = %#v", created)
	}
	updated, err := service.UpdateSource(context.Background(), created.ID, SourceInput{Name: "社区插件", URL: "https://plugins.example/v2/catalog.json"})
	if err != nil || updated.Name != "社区插件" {
		t.Fatalf("UpdateSource() = %#v, %v", updated, err)
	}
	if err := service.DeleteSource(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(Query{SourceID: created.ID}); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("deleted source List() error = %v", err)
	}
	if err := service.DeleteSource(context.Background(), OfficialSourceID); !errors.Is(err, ErrSourceImmutable) {
		t.Fatalf("official DeleteSource() error = %v", err)
	}
}

func TestServiceRejectsInvalidCustomSource(t *testing.T) {
	service := newTestService(t, emptyCatalog{}, nil, newMemoryRepository(nil), staticCatalogTransport(nil))
	for _, input := range []SourceInput{
		{Name: "", URL: "https://plugins.example/catalog.json"},
		{Name: "本机", URL: "https://127.0.0.1/catalog.json"},
		{Name: "非加密", URL: "http://plugins.example/catalog.json"},
	} {
		if _, err := service.CreateSource(context.Background(), input); !errors.Is(err, ErrSourceInvalid) {
			t.Fatalf("CreateSource(%#v) error = %v", input, err)
		}
	}
}

func TestServiceInspectionRequiresConfirmationOnlyForNewTrust(t *testing.T) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	payload := catalogJSON(platform, "0.4.0", "0.4.0")
	repository := newMemoryRepository(payload)
	installer := &stubInstaller{inspection: plugins.InstallInspection{
		InspectionID:  strings.Repeat("a", 64),
		ExpiresAt:     time.Now().Add(time.Minute),
		PackageSHA256: strings.Repeat("b", 64),
		PluginID:      "raylea.echo",
		PluginName:    "Echo",
		Version:       "0.4.0",
		Permissions:   map[string]plugins.PermissionGrant{"message.send": {}},
	}}
	service := newTestService(t, emptyCatalog{}, installer, repository, staticCatalogTransport(payload))
	first, err := service.Inspect(context.Background(), InspectionRequest{SourceID: OfficialSourceID, PluginID: "raylea.echo"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.ConfirmationRequired || len(first.ConfirmationReasons) != 1 || first.ConfirmationReasons[0] != "first_install" {
		t.Fatalf("first inspection = %#v", first)
	}
	if _, err := service.Install(context.Background(), InstallRequest{
		PluginID:      "raylea.echo",
		InspectionID:  first.Inspection.InspectionID,
		PackageSHA256: first.Inspection.PackageSHA256,
	}); !errors.Is(err, plugins.ErrTrustedCodeConfirmation) {
		t.Fatalf("unconfirmed install error = %v", err)
	}

	installed := fixedCatalog{snapshot: plugins.Snapshot{
		PluginID: "raylea.echo", Version: "0.3.0", PackageSourceType: "catalog", PackageSourceRef: OfficialSourceID,
		Permissions: map[string]plugins.PermissionGrant{"message.send": {}},
	}}
	installer = &stubInstaller{inspection: installer.inspection}
	service = newTestService(t, installed, installer, repository, staticCatalogTransport(payload))
	update, err := service.Inspect(context.Background(), InspectionRequest{SourceID: OfficialSourceID, PluginID: "raylea.echo"})
	if err != nil {
		t.Fatal(err)
	}
	if update.ConfirmationRequired {
		t.Fatalf("same-source update unexpectedly requires confirmation: %#v", update)
	}
	if _, err := service.Install(context.Background(), InstallRequest{
		PluginID:      "raylea.echo",
		InspectionID:  update.Inspection.InspectionID,
		PackageSHA256: update.Inspection.PackageSHA256,
	}); err != nil {
		t.Fatalf("same-source update failed: %v", err)
	}
}

func TestPermissionsExpanded(t *testing.T) {
	current := map[string]plugins.PermissionGrant{"thirdparty.account.read": {Platforms: []string{"bilibili", "weibo"}}}
	if permissionsExpanded(current, map[string]plugins.PermissionGrant{"thirdparty.account.read": {Platforms: []string{"bilibili"}}}) {
		t.Fatal("permission reduction was classified as expansion")
	}
	if !permissionsExpanded(current, map[string]plugins.PermissionGrant{"thirdparty.account.read": {Platforms: []string{"bilibili", "douyin"}}}) {
		t.Fatal("new platform was not classified as expansion")
	}
	if !permissionsExpanded(current, map[string]plugins.PermissionGrant{"message.send": {}}) {
		t.Fatal("new permission was not classified as expansion")
	}
}

func newTestService(t *testing.T, installed plugins.CatalogView, installer Installer, repository Repository, transport http.RoundTripper) *Service {
	t.Helper()
	service, err := New(context.Background(), installed, installer, repository, Options{
		CoreVersion: "0.4.0",
		HTTPClient:  &http.Client{Transport: transport},
		Now:         func() time.Time { return time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func catalogJSON(platform, version, minCoreVersion string) []byte {
	return []byte(`{
  "catalog_version":"2",
  "entries":[{
    "id":"raylea.echo",
    "name":"Echo",
    "summary":"Echo messages",
    "publisher":{"id":"rayleabot","name":"RayleaBot"},
    "repository_url":"https://github.com/RayleaBot/plugin-echo",
    "license":"MIT",
    "keywords":["echo"],
    "recommended":true,
    "current_release":{
      "version":"` + version + `",
      "published_at":"2026-09-03T00:00:00Z",
      "min_core_version":"` + minCoreVersion + `",
      "assets":[{"platform":"` + platform + `","url":"https://downloads.example/echo.zip","archive_sha256":"` + strings.Repeat("a", 64) + `"}]
    }
  }]
}`)
}

type memoryRepository struct {
	mu       sync.Mutex
	sources  map[string]Source
	catalogs map[string]CachedCatalog
}

func newMemoryRepository(officialPayload []byte) *memoryRepository {
	repository := &memoryRepository{
		sources: map[string]Source{OfficialSourceID: {
			ID: OfficialSourceID, Name: "RayleaBot 官方插件", URL: "https://plugins.example/official.json", Official: true,
		}},
		catalogs: map[string]CachedCatalog{},
	}
	if len(officialPayload) > 0 {
		repository.catalogs[OfficialSourceID] = CachedCatalog{SourceID: OfficialSourceID, Payload: officialPayload, RefreshedAt: time.Now()}
	}
	return repository
}

func (r *memoryRepository) ListSources(context.Context) ([]Source, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]Source, 0, len(r.sources))
	for _, source := range r.sources {
		items = append(items, source)
	}
	return items, nil
}

func (r *memoryRepository) CreateSource(_ context.Context, source Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sources {
		if existing.URL == source.URL {
			return ErrSourceConflict
		}
	}
	r.sources[source.ID] = source
	return nil
}

func (r *memoryRepository) UpdateSource(_ context.Context, source Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sources[source.ID]; !ok {
		return ErrSourceNotFound
	}
	r.sources[source.ID] = source
	return nil
}

func (r *memoryRepository) DeleteSource(_ context.Context, sourceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, sourceID)
	delete(r.catalogs, sourceID)
	return nil
}

func (r *memoryRepository) LoadCatalogs(context.Context) ([]CachedCatalog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]CachedCatalog, 0, len(r.catalogs))
	for _, catalog := range r.catalogs {
		items = append(items, catalog)
	}
	return items, nil
}

func (r *memoryRepository) SaveCatalog(_ context.Context, catalog CachedCatalog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.catalogs[catalog.SourceID] = catalog
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func staticCatalogTransport(payload []byte) http.RoundTripper {
	return roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	})
}

type stubInstaller struct {
	inspection plugins.InstallInspection
	accepted   plugins.InstallAcceptance
}

func (s *stubInstaller) Inspect(_ context.Context, request plugins.InstallRequest) (plugins.InstallInspection, error) {
	inspection := s.inspection
	inspection.SourceType = request.SourceType
	inspection.Source = request.Source
	return inspection, nil
}

func (s *stubInstaller) Accept(_ context.Context, acceptance plugins.InstallAcceptance) (string, error) {
	s.accepted = acceptance
	return "task-1", nil
}

func (*stubInstaller) Cancel(string) bool { return false }
func (*stubInstaller) Close() error       { return nil }

type emptyCatalog struct{}

func (emptyCatalog) List() []plugins.Snapshot            { return nil }
func (emptyCatalog) Get(string) (plugins.Snapshot, bool) { return plugins.Snapshot{}, false }
func (emptyCatalog) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}

type fixedCatalog struct{ snapshot plugins.Snapshot }

func (c fixedCatalog) List() []plugins.Snapshot { return []plugins.Snapshot{c.snapshot} }
func (c fixedCatalog) Get(id string) (plugins.Snapshot, bool) {
	return c.snapshot, id == c.snapshot.PluginID
}
func (c fixedCatalog) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return c.snapshot, nil
}
