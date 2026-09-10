package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/go-chi/chi/v5"
)

func TestPluginIconServesOnlyDeclaredImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`)
	if err := os.WriteFile(filepath.Join(root, "icon.svg"), svg, 0600); err != nil {
		t.Fatal(err)
	}
	catalog := plugincatalog.New([]plugins.Snapshot{{PluginID: "icon-test", Valid: true, RegistrationState: "installed", PackageRootPath: root, Icon: "icon.svg"}})
	router := pluginRouter(t, catalog)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/icon-test/icon?path=../outside", nil))
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), svg) {
		t.Fatalf("declared icon response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for name, want := range map[string]string{
		"Content-Type": "image/svg+xml", "Cache-Control": "private, no-store",
		"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; sandbox",
		"X-Content-Type-Options":  "nosniff", "Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestPluginIconUnavailableDoesNotExposeFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, body := range map[string][]byte{
		"private.txt": []byte("private fixture content"),
		"fake.svg":    []byte(`<html><script>window.fixture = true</script></html>`),
		"broken.svg":  []byte(`<svg><path>`),
		"doctype.svg": []byte(`<!DOCTYPE svg SYSTEM "file:///fixture"><svg/>`),
		"large.svg":   bytes.Repeat([]byte("x"), pluginIconMaxBytes+1),
	} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"", ".", "../private.txt", "/private.txt", `C:\fixture.txt`, "icon.svg:stream", "missing.svg", "private.txt", "fake.svg", "broken.svg", "doctype.svg", "large.svg"} {
		t.Run(name, func(t *testing.T) {
			catalog := plugincatalog.New([]plugins.Snapshot{{PluginID: "icon-test", Valid: true, RegistrationState: "installed", PackageRootPath: root, Icon: name}})
			recorder := httptest.NewRecorder()
			pluginRouter(t, catalog).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/icon-test/icon", nil))
			assertPluginIconMissing(t, recorder, root)
		})
	}
	for _, snapshot := range []plugins.Snapshot{
		{PluginID: "another-plugin"},
		{PluginID: "icon-test", Valid: false, RegistrationState: "installed", PackageRootPath: root, Icon: "private.txt"},
	} {
		recorder := httptest.NewRecorder()
		pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/icon-test/icon", nil))
		assertPluginIconMissing(t, recorder, root)
	}
}

func TestPluginIconRejectsSymlinkOutsidePackage(t *testing.T) {
	t.Parallel()
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside.svg")
	if err := os.WriteFile(outside, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "icon.svg")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	snapshot := plugins.Snapshot{PluginID: "icon-test", Valid: true, RegistrationState: "installed", PackageRootPath: root, Icon: "icon.svg"}
	recorder := httptest.NewRecorder()
	pluginRouter(t, plugincatalog.New([]plugins.Snapshot{snapshot})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/icon-test/icon", nil))
	assertPluginIconMissing(t, recorder, outside)
}

func TestPluginIconRouteRequiresSession(t *testing.T) {
	t.Parallel()
	manager, err := auth.NewManager(auth.Config{SessionTTLDays: 1, MaxSessions: 3})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterRoutes(router, RouteDeps{ProtectedRoutes: []ProtectedRouteModule{ProtectedRouteFunc(func(r chi.Router) {
		registerPluginReadRoutes(r, plugincatalog.New(nil))
	})}}, RequireAuth(manager))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins/icon-test/icon", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed icon status = %d", recorder.Code)
	}
}

func assertPluginIconMissing(t *testing.T, recorder *httptest.ResponseRecorder, privatePath string) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNotFound || body.Error.Code != "platform.resource_not_found" {
		t.Fatalf("unavailable icon status=%d code=%s", recorder.Code, body.Error.Code)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(privatePath)) || bytes.Contains(recorder.Body.Bytes(), []byte("private fixture content")) {
		t.Fatal("icon failure exposed file content or path")
	}
}
