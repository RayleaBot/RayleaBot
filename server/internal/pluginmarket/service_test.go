package pluginmarket

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginartifact "github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

func TestBootstrapCatalogListsFormerBuiltinsAsUnpublishedOfficialEntries(t *testing.T) {
	t.Parallel()

	service, err := New(emptyCatalog{}, nil, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result := service.List(Query{Limit: 100})
	if result.Total != 4 {
		t.Fatalf("List().Total = %d, want 4", result.Total)
	}
	if !result.Catalog.Verified || result.Catalog.Source != "embedded" {
		t.Fatalf("List().Catalog = %#v, want verified embedded catalog", result.Catalog)
	}
	for _, item := range result.Items {
		if !item.Publisher.Verified || !item.Recommended || item.InstallState != "unpublished" {
			t.Fatalf("catalog item = %#v, want verified recommended unpublished entry", item)
		}
	}
}

func TestRefreshAcceptsOnlyExactCatalogBytesSignedByTrustedKey(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	catalogBytes := mustCatalogJSON(t, Catalog{
		CatalogVersion: "1",
		GeneratedAt:    "2026-08-04T00:00:00Z",
		Entries:        []Entry{testEntry(t, nil)},
	})
	digest := sha256.Sum256(catalogBytes)
	envelopeBytes, err := json.Marshal(SignatureEnvelope{
		SignatureVersion: 1,
		Algorithm:        "ed25519",
		CatalogSHA256:    hex.EncodeToString(digest[:]),
		KeyID:            "store-test",
		Signatures: []Signature{{
			KeyID:     "store-test",
			Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(privateKey, catalogBytes)),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/catalog.json":
			_, _ = w.Write(catalogBytes)
		case "/catalog.sig.json":
			_, _ = w.Write(envelopeBytes)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	service, err := New(emptyCatalog{}, nil, Options{
		CatalogURL:      server.URL + "/catalog.json",
		SignatureURL:    server.URL + "/catalog.sig.json",
		TrustedKeysSpec: "store-test=" + base64.StdEncoding.EncodeToString(publicKey),
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	status, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if status.Source != "remote" || !status.Verified || len(status.TrustedKeyIDs) != 1 || status.TrustedKeyIDs[0] != "store-test" {
		t.Fatalf("Refresh() status = %#v", status)
	}

	envelopeBytes[0] ^= 1
	if _, err := service.Refresh(context.Background()); ErrorCode(err) != CodeCatalogUnavailable {
		t.Fatalf("tampered Refresh() error = %v, want %s", err, CodeCatalogUnavailable)
	}
	if service.List(Query{}).Catalog.Source != "remote" {
		t.Fatal("failed refresh replaced the last verified catalog")
	}
}

func TestSignatureEnvelopeRequiresUniqueSignaturesAndDeclaredPrimary(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	catalogBytes := mustCatalogJSON(t, Catalog{
		CatalogVersion: "1",
		GeneratedAt:    "2026-08-04T00:00:00Z",
		Entries:        []Entry{testEntry(t, nil)},
	})
	digest := sha256.Sum256(catalogBytes)
	valid := Signature{
		KeyID:     "store-test",
		Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(privateKey, catalogBytes)),
	}
	service, err := New(emptyCatalog{}, nil, Options{
		TrustedKeysSpec: "store-test=" + base64.StdEncoding.EncodeToString(publicKey),
	})
	if err != nil {
		t.Fatal(err)
	}

	for name, envelope := range map[string]SignatureEnvelope{
		"missing primary": {
			SignatureVersion: 1, Algorithm: "ed25519", CatalogSHA256: hex.EncodeToString(digest[:]),
			KeyID: "store-other", Signatures: []Signature{valid},
		},
		"duplicate key": {
			SignatureVersion: 1, Algorithm: "ed25519", CatalogSHA256: hex.EncodeToString(digest[:]),
			KeyID: "store-test", Signatures: []Signature{valid, valid},
		},
	} {
		t.Run(name, func(t *testing.T) {
			envelopeBytes, marshalErr := json.Marshal(envelope)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, verifyErr := service.verifySignature(catalogBytes, envelopeBytes); verifyErr == nil {
				t.Fatal("verifySignature() accepted an invalid signature envelope")
			}
		})
	}
}

func TestRefreshRejectsOlderCatalogAndKeepsLastVerifiedSnapshot(t *testing.T) {
	t.Parallel()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	catalogBytes := mustCatalogJSON(t, Catalog{
		CatalogVersion: "1",
		GeneratedAt:    "2026-08-03T23:59:59Z",
		Entries:        []Entry{testEntry(t, nil)},
	})
	digest := sha256.Sum256(catalogBytes)
	envelopeBytes, err := json.Marshal(SignatureEnvelope{
		SignatureVersion: 1,
		Algorithm:        "ed25519",
		CatalogSHA256:    hex.EncodeToString(digest[:]),
		KeyID:            "store-test",
		Signatures: []Signature{{
			KeyID:     "store-test",
			Signature: base64.URLEncoding.EncodeToString(ed25519.Sign(privateKey, catalogBytes)),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/catalog.json" {
			_, _ = w.Write(catalogBytes)
			return
		}
		_, _ = w.Write(envelopeBytes)
	}))
	defer server.Close()

	service, err := New(emptyCatalog{}, nil, Options{
		CatalogURL:      server.URL + "/catalog.json",
		SignatureURL:    server.URL + "/catalog.sig.json",
		TrustedKeysSpec: "store-test=" + base64.StdEncoding.EncodeToString(publicKey),
		HTTPClient:      server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	status, refreshErr := service.Refresh(context.Background())
	if ErrorCode(refreshErr) != CodeCatalogUnavailable {
		t.Fatalf("Refresh() error = %v, want %s", refreshErr, CodeCatalogUnavailable)
	}
	if status.Source != "embedded" || service.List(Query{}).Catalog.Source != "embedded" {
		t.Fatalf("older catalog replaced bootstrap snapshot: %#v", status)
	}
}

func TestInstallFreezesCatalogIdentityAndDigestsIntoUnifiedInstallerRequest(t *testing.T) {
	t.Parallel()

	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := strings.Repeat("a", 64)
	archiveHash := strings.Repeat("b", 64)
	installer := &recordingInstaller{}
	service, err := New(emptyCatalog{}, installer, Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	entry := testEntry(t, []Release{{
		Version:        "0.2.0",
		PublishedAt:    "2026-08-04T00:00:00Z",
		MinCoreVersion: "0.1.0",
		Assets: []Asset{{
			Platform: platform, URL: "https://downloads.example/plugin.zip", ArchiveSizeBytes: 32,
			ArchiveSHA256: archiveHash, ManifestSHA256: manifestHash,
		}},
	}})
	service.snapshot = newCatalogSnapshot(Catalog{
		CatalogVersion: "1", GeneratedAt: "2026-08-04T00:00:00Z", Entries: []Entry{entry},
	}, "remote", strings.Repeat("c", 64), []string{"store-test"})
	installer.inspection = plugins.InstallInspection{
		InspectionID: "inspection-1", PackageSHA256: strings.Repeat("d", 64),
		PluginID: entry.ID, Version: "0.2.0", Artifact: plugins.ArtifactInspection{ManifestSHA256: manifestHash},
	}

	taskID, err := service.Install(context.Background(), InstallRequest{PluginID: entry.ID, TrustedCodeConfirmed: true})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if taskID != "task-store-install" {
		t.Fatalf("Install() task id = %q", taskID)
	}
	request := installer.accepted
	if request.SourceType != "catalog" || request.ResolvedSourceType != "remote_url" || request.ResolvedSource != "https://downloads.example/plugin.zip" {
		t.Fatalf("installer source = %#v", request)
	}
	if request.ExpectedArchiveSize != 32 || request.ExpectedArchiveSHA256 != archiveHash || request.ExpectedManifestSHA256 != manifestHash || !request.PublisherVerified {
		t.Fatalf("installer trust metadata = %#v", request)
	}
}

func TestListSelectsLatestCompatibleReleaseForCurrentPlatform(t *testing.T) {
	t.Parallel()

	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(emptyCatalog{}, nil, Options{CoreVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	entry := testEntry(t, []Release{
		{
			Version: "0.3.0", PublishedAt: "2026-08-04T00:00:00Z", MinCoreVersion: "0.3.0",
			Assets: []Asset{{Platform: platform, URL: "https://downloads.example/compatible.zip", ArchiveSizeBytes: 1, ArchiveSHA256: strings.Repeat("a", 64), ManifestSHA256: strings.Repeat("b", 64)}},
		},
		{
			Version: "0.4.0", PublishedAt: "2026-08-04T01:00:00Z", MinCoreVersion: "0.4.0",
			Assets: []Asset{{Platform: platform, URL: "https://downloads.example/future.zip", ArchiveSizeBytes: 1, ArchiveSHA256: strings.Repeat("c", 64), ManifestSHA256: strings.Repeat("d", 64)}},
		},
	})
	service.snapshot = newCatalogSnapshot(Catalog{
		CatalogVersion: "1", GeneratedAt: "2026-08-04T02:00:00Z", Entries: []Entry{entry},
	}, "remote", strings.Repeat("e", 64), []string{"store-test"})

	result := service.List(Query{Limit: 10})
	if len(result.Items) != 1 || result.Items[0].LatestRelease == nil {
		t.Fatalf("List() = %#v", result)
	}
	item := result.Items[0]
	if item.LatestRelease.Version != "0.3.0" || item.InstallState != "available" {
		t.Fatalf("List() item = %#v, want latest compatible 0.3.0", item)
	}
}

func TestListDoesNotPresentYankedOnlyEntryAsUnpublished(t *testing.T) {
	t.Parallel()

	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(emptyCatalog{}, nil, Options{CoreVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	entry := testEntry(t, []Release{{
		Version: "0.2.0", PublishedAt: "2026-08-04T00:00:00Z", MinCoreVersion: "0.1.0", Yanked: true,
		Assets: []Asset{{Platform: platform, URL: "https://downloads.example/yanked.zip", ArchiveSizeBytes: 1, ArchiveSHA256: strings.Repeat("a", 64), ManifestSHA256: strings.Repeat("b", 64)}},
	}})
	service.snapshot = newCatalogSnapshot(Catalog{
		CatalogVersion: "1", GeneratedAt: "2026-08-04T02:00:00Z", Entries: []Entry{entry},
	}, "remote", strings.Repeat("c", 64), []string{"store-test"})

	result := service.List(Query{Limit: 10})
	if len(result.Items) != 1 || result.Items[0].LatestRelease == nil || !result.Items[0].LatestRelease.Yanked || result.Items[0].InstallState != "incompatible" {
		t.Fatalf("List() = %#v, want yanked incompatible release", result)
	}
}

func TestCompareSemverOrdersPrereleasesAndIgnoresBuildMetadata(t *testing.T) {
	t.Parallel()

	cases := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.0.0-alpha", right: "1.0.0", want: -1},
		{left: "1.0.0-alpha.1", right: "1.0.0-alpha.beta", want: -1},
		{left: "1.0.0-beta.11", right: "1.0.0-rc.1", want: -1},
		{left: "1.0.0+build.2", right: "1.0.0+build.1", want: 0},
		{left: "2.0.0", right: "1.999.999", want: 1},
	}
	for _, testCase := range cases {
		got := compareSemver(testCase.left, testCase.right)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		if got != testCase.want {
			t.Fatalf("compareSemver(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
		}
	}
}

func testEntry(t *testing.T, releases []Release) Entry {
	t.Helper()
	if releases == nil {
		releases = []Release{}
	}
	return Entry{
		ID: "raylea.test", Name: "Test", Summary: "Test plugin", Description: "Test plugin description",
		Publisher:     Publisher{ID: "rayleabot", Name: "RayleaBot", Verified: true},
		RepositoryURL: "https://github.com/RayleaBot/plugin-test", License: "MIT", Keywords: []string{"test"},
		Recommended: true, Releases: releases,
	}
}

func mustCatalogJSON(t *testing.T, catalog Catalog) []byte {
	t.Helper()
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

type emptyCatalog struct{}

func (emptyCatalog) List() []plugins.Snapshot            { return nil }
func (emptyCatalog) Get(string) (plugins.Snapshot, bool) { return plugins.Snapshot{}, false }
func (emptyCatalog) SetDesiredState(string, string) (plugins.Snapshot, error) {
	return plugins.Snapshot{}, nil
}

type recordingInstaller struct {
	inspection plugins.InstallInspection
	inspected  plugins.InstallRequest
	accepted   plugins.InstallRequest
}

func (i *recordingInstaller) Inspect(_ context.Context, request plugins.InstallRequest) (plugins.InstallInspection, error) {
	i.inspected = request
	return i.inspection, nil
}

func (i *recordingInstaller) Accept(_ context.Context, request plugins.InstallRequest) (string, error) {
	i.accepted = request
	return "task-store-install", nil
}

func (*recordingInstaller) Cancel(string) bool { return false }
func (*recordingInstaller) Close() error       { return nil }
