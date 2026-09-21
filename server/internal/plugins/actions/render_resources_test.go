package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestPrefetchRenderImageResourcesUsesRefererAndFallbackURL(t *testing.T) {
	t.Parallel()

	content := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte("fixture-render-resource")...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Referer") != "https://weibo.com/" {
			t.Errorf("Referer = %q", request.Header.Get("Referer"))
		}
		if !strings.Contains(request.Header.Get("User-Agent"), "Chrome/152") {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		if request.URL.Path == "/large/image.jpg" {
			http.NotFound(w, request)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	defer server.Close()

	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-request",
		Action: plugins.Action{RenderResources: []plugins.RenderImageResource{{
			ID:           "media-0",
			URL:          server.URL + "/large/image.jpg",
			FallbackURLs: []string{server.URL + "/mw2000/image.jpg"},
			Referer:      "https://weibo.com/",
		}}},
	})
	if err != nil {
		t.Fatalf("prefetchRenderImageResources: %v", err)
	}
	if len(resources) != 1 {
		cleanup()
		t.Fatalf("resources = %#v", resources)
	}
	resource := resources[0]
	if resource.ID != "media-0" || resource.MIME != "image/png" || resource.Size != int64(len(content)) {
		cleanup()
		t.Fatalf("resource = %#v", resource)
	}
	digest := sha256.Sum256(content)
	if resource.SHA256 != hex.EncodeToString(digest[:]) {
		cleanup()
		t.Fatalf("SHA256 = %q", resource.SHA256)
	}
	stored, readErr := os.ReadFile(resource.Path)
	if readErr != nil || string(stored) != string(content) {
		cleanup()
		t.Fatalf("stored resource = %q, err=%v", stored, readErr)
	}
	path := resource.Path
	cleanup()
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("resource workspace was not removed: %v", statErr)
	}
}

func TestPrefetchRenderImageResourcesAllowsPrivateHost(t *testing.T) {
	t.Parallel()

	content := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte("fixture-suffix-host")...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(content)
	}))
	defer server.Close()

	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-suffix",
		Action: plugins.Action{RenderResources: []plugins.RenderImageResource{{
			ID:  "media-0",
			URL: server.URL + "/cover.png",
		}}},
	})
	if err != nil {
		t.Fatalf("prefetchRenderImageResources: %v", err)
	}
	defer cleanup()
	if len(resources) != 1 || resources[0].ID != "media-0" {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestPrefetchRenderImageResourcesRejectsNonHTTPSRedirect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://example.com/image.jpg")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	_, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-redirect-scope",
		Action: plugins.Action{RenderResources: []plugins.RenderImageResource{{
			ID:  "media-0",
			URL: server.URL + "/image.jpg",
		}}},
	})
	cleanup()
	var runtimeErr *plugins.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("error = %#v", err)
	}
}

func TestPrefetchRenderImageResourcesReadsCallerDataDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte("fixture-local-resource")...)
	writeResourceFixture(t, filepath.Join(root, "plugin.render", "assets", "芙宁娜", "face.png"), content)
	writeResourceFixture(t, filepath.Join(root, "plugin.render", "assets", "note.txt"), []byte("not an image"))
	writeResourceFixture(t, filepath.Join(root, "plugin.render", "assets", "font.woff2"), append([]byte("wOF2"), make([]byte, 60)...))
	writeResourceFixture(t, filepath.Join(root, "plugin.other", "face.png"), content)
	linked := os.Symlink(filepath.Join(root, "plugin.other"), filepath.Join(root, "plugin.render", "other")) == nil

	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{PluginDataRoot: root}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-local",
		Action: plugins.Action{RenderResources: []plugins.RenderImageResource{
			{ID: "face", Path: "assets/芙宁娜/face.png"},
			{ID: "missing", Path: "assets/missing.png"},
			{ID: "text", Path: "assets/note.txt"},
			{ID: "linked", Path: "other/face.png"},
			{ID: "font", Path: "assets/font.woff2"},
		}},
	})
	if err != nil {
		t.Fatalf("prefetchRenderImageResources: %v", err)
	}
	defer cleanup()
	if len(resources) != 2 || resources[0].ID != "face" || resources[0].MIME != "image/png" || resources[0].Size != int64(len(content)) || resources[1].MIME != "font/woff2" {
		t.Fatalf("resources = %#v (link created: %v)", resources, linked)
	}
	digest := sha256.Sum256(content)
	if resources[0].SHA256 != hex.EncodeToString(digest[:]) || filepath.Dir(resources[0].Path) == filepath.Join(root, "plugin.render", "assets", "芙宁娜") {
		t.Fatalf("resource was not copied into the workspace: %#v", resources[0])
	}
}

func writeResourceFixture(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
