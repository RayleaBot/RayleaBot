package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
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

	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{
		CurrentConfig: func() config.Config {
			return config.Config{HTTP: config.HTTPConfig{
				TimeoutSeconds:    5,
				AllowPrivateHosts: []string{"127.0.0.1"},
			}}
		},
		Permissions: stubHTTPActionPermissions{
			permissions: map[string]bool{"render.image": true, "http.request": true},
		},
	}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-request",
		Action: pluginruntime.Action{RenderResources: []pluginruntime.RenderImageResource{{
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

func TestPrefetchRenderImageResourcesAllowsConfiguredPrivateHost(t *testing.T) {
	t.Parallel()

	content := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte("fixture-suffix-host")...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(content)
	}))
	defer server.Close()

	resources, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{
		CurrentConfig: func() config.Config {
			return config.Config{HTTP: config.HTTPConfig{
				TimeoutSeconds:    5,
				AllowPrivateHosts: []string{"127.0.0.1"},
			}}
		},
		Permissions: stubHTTPActionPermissions{
			permissions: map[string]bool{"render.image": true, "http.request": true},
		},
	}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-suffix",
		Action: pluginruntime.Action{RenderResources: []pluginruntime.RenderImageResource{{
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

func TestPrefetchRenderImageResourcesRejectsPrivateHostWithoutServerAllowlist(t *testing.T) {
	t.Parallel()

	_, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{
		CurrentConfig: func() config.Config {
			return config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 1}}
		},
		Permissions: stubHTTPActionPermissions{
			permissions: map[string]bool{"render.image": true, "http.request": true},
		},
	}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-scope",
		Action: pluginruntime.Action{RenderResources: []pluginruntime.RenderImageResource{{
			ID:  "media-0",
			URL: "https://127.0.0.1/image.jpg",
		}}},
	})
	cleanup()
	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("error = %#v", err)
	}
}

func TestPrefetchRenderImageResourcesRevalidatesRedirectSafety(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://example.com/image.jpg")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	_, cleanup, err := prefetchRenderImageResources(context.Background(), Deps{
		CurrentConfig: func() config.Config {
			return config.Config{HTTP: config.HTTPConfig{
				TimeoutSeconds:    5,
				AllowPrivateHosts: []string{"127.0.0.1"},
			}}
		},
		Permissions: stubHTTPActionPermissions{
			permissions: map[string]bool{"render.image": true, "http.request": true},
		},
	}, ActionRequest{
		PluginID:  "plugin.render",
		RequestID: "render-resource-redirect-scope",
		Action: pluginruntime.Action{RenderResources: []pluginruntime.RenderImageResource{{
			ID:  "media-0",
			URL: server.URL + "/image.jpg",
		}}},
	})
	cleanup()
	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "platform.invalid_request" {
		t.Fatalf("error = %#v", err)
	}
}
