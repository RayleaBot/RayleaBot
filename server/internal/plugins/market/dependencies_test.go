package market

import (
	"bytes"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestStoreProjectsDirectDependencies(t *testing.T) {
	document := storeSnapshotFixture(t)
	base := cloneEntry(document.Entries[0])
	dependencies := []plugins.Dependency{
		{ID: "installed", Requirement: "required", Reason: "resource"},
		{ID: "available", Requirement: "required"},
		{ID: "update", Requirement: "recommended"},
		{ID: "unpublished", Requirement: "recommended"},
		{ID: "incompatible", Requirement: "required"},
		{ID: "no-asset", Requirement: "recommended"},
		{ID: "absent", Requirement: "required"},
	}
	document.Entries[0].CurrentRelease.Dependencies = dependencies
	for _, dependency := range dependencies[:len(dependencies)-1] {
		entry := cloneEntry(base)
		entry.ID, entry.Name = dependency.ID, "Catalog "+dependency.ID
		switch dependency.ID {
		case "unpublished":
			entry.CurrentRelease = nil
		case "incompatible":
			entry.CurrentRelease.MinCoreVersion = "999.0.0"
		case "no-asset":
			entry.CurrentRelease.Assets[0].Platform = "future-platform"
		}
		document.Entries = append(document.Entries, entry)
	}
	installed := plugincatalog.New([]plugins.Snapshot{
		{PluginID: "installed", Name: "Installed name", Valid: true, RegistrationState: plugins.RegistrationStateInstalled, DesiredState: plugins.DesiredStateDisabled},
		{PluginID: "available", Name: "Invalid name", RegistrationState: plugins.RegistrationStateInstalled},
		{PluginID: "update", Name: "Unregistered name", Valid: true, Version: "0.3.0"},
	})
	payload := storeSnapshotPayload(t, document)
	repository := newMemoryRepository(payload)
	other := cloneEntry(base)
	other.ID, other.Name = "absent", "Other source name"
	repository.sources["other"] = Source{ID: "other", Name: "Other", URL: "https://other.example/catalog.json"}
	repository.catalogs["other"] = CachedCatalog{SourceID: "other", Payload: storeSnapshotPayload(t, Catalog{CatalogVersion: "2", Entries: []Entry{other}})}
	service := newTestService(t, installed, nil, repository, staticCatalogTransport(payload))
	list, err := service.List(Query{Text: "raylea.echo"})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("List() = %+v, %v", list, err)
	}
	detail, ok := service.Get(OfficialSourceID, "raylea.echo")
	if !ok {
		t.Fatal("missing detail")
	}
	want := []DependencyView{
		{ID: "installed", Name: "Installed name", Requirement: "required", Reason: "resource", State: "installed"},
		{ID: "available", Name: "Catalog available", Requirement: "required", State: "installable"},
		{ID: "update", Name: "Catalog update", Requirement: "recommended", State: "installable"},
		{ID: "unpublished", Name: "Catalog unpublished", Requirement: "recommended", State: "unavailable"},
		{ID: "incompatible", Name: "Catalog incompatible", Requirement: "required", State: "unavailable"},
		{ID: "no-asset", Name: "Catalog no-asset", Requirement: "recommended", State: "unavailable"},
		{ID: "absent", Name: "absent", Requirement: "required", State: "unavailable"},
	}
	for _, release := range []*ReleaseView{list.Items[0].LatestRelease, detail.Plugin.LatestRelease, detail.CurrentRelease} {
		if release == nil || !reflect.DeepEqual(release.Dependencies, want) {
			t.Fatalf("release = %+v", release)
		}
	}
}

func TestStoreRejectsMissingDependenciesBeforeDownload(t *testing.T) {
	document := storeSnapshotFixture(t)
	document.Entries[0].CurrentRelease.Dependencies = []plugins.Dependency{
		{ID: "second", Requirement: "required"},
		{ID: "optional", Requirement: "recommended"},
		{ID: "first", Requirement: "required"},
	}
	payload := storeSnapshotPayload(t, document)
	installer := &stubInstaller{}
	service := newTestService(t, emptyCatalog{}, installer, newMemoryRepository(payload), roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("missing dependencies triggered a download")
		return nil, errors.New("unexpected download")
	}))
	_, err := service.Install(t.Context(), InstallRequest{PluginID: "raylea.echo", TrustedCodeConfirmed: true})
	var missing *plugins.DependencyMissingError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Details(), map[string]any{"plugin_ids": []string{"second", "first"}}) {
		t.Fatalf("Install() error = %v", err)
	}
	if installer.accepted.Source != "" {
		t.Fatalf("installer accepted: %+v", installer.accepted)
	}
}

func TestCatalogIgnoresUnknownDependencyRequirements(t *testing.T) {
	document := storeSnapshotFixture(t)
	payload := bytes.Replace(storeSnapshotPayload(t, document), []byte(`"assets":`), []byte(`"dependencies":[{"requirement":"future","id":null,"reason":false},{"id":"optional","requirement":"recommended","reason":"hint","future":true}],"assets":`), 1)
	installer := &stubInstaller{}
	service := newTestService(t, emptyCatalog{}, installer, newMemoryRepository(payload), staticCatalogTransport(payload))
	detail, ok := service.Get(OfficialSourceID, "raylea.echo")
	if !ok || !reflect.DeepEqual(detail.CurrentRelease.Dependencies, []DependencyView{{ID: "optional", Name: "optional", Requirement: "recommended", Reason: "hint", State: "unavailable"}}) {
		t.Fatalf("detail = %+v", detail)
	}
	if _, err := service.Install(t.Context(), InstallRequest{PluginID: "raylea.echo", TrustedCodeConfirmed: true}); err != nil {
		t.Fatal(err)
	}
	if installer.accepted.Source == "" {
		t.Fatal("installer was not called")
	}
}
