package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCacheKeyIncludesPrefetchedResourceDigest(t *testing.T) {
	t.Parallel()

	request := Request{
		Template: "plugin.example.card",
		Theme:    "default",
		Output:   "png",
		Resources: []RenderResource{{
			ID: "media-0", MIME: "image/jpeg", SHA256: strings.Repeat("a", 64), Size: 128,
		}},
	}
	first := BuildCacheKey(request, "1", "source", "assets", 100, []byte(`{"title":"fixture"}`))
	request.Resources[0].SHA256 = strings.Repeat("b", 64)
	second := BuildCacheKey(request, "1", "source", "assets", 100, []byte(`{"title":"fixture"}`))
	if first == second {
		t.Fatalf("cache key did not change with resource bytes: %q", first)
	}
}

func TestWriteTemporaryRenderDocumentMaterializesAndCleansResources(t *testing.T) {
	t.Parallel()

	content := []byte("fixture-image-bytes")
	source := filepath.Join(t.TempDir(), "source.jpg")
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	renderURL, resourceURLs, cleanup, err := writeTemporaryRenderDocument(
		`<html><body><img data-render-resource="media-0"></body></html>`,
		"",
		[]RenderResource{{
			ID: "media-0", Path: source, MIME: "image/jpeg", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(content)),
		}},
	)
	if err != nil {
		t.Fatalf("writeTemporaryRenderDocument: %v", err)
	}
	parsedDocument, err := url.Parse(renderURL)
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	documentPath := windowsFileURLPath(parsedDocument)
	parsedResource, err := url.Parse(resourceURLs["media-0"])
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	resourcePath := windowsFileURLPath(parsedResource)
	materialized, readErr := os.ReadFile(resourcePath)
	if readErr != nil || string(materialized) != string(content) {
		cleanup()
		t.Fatalf("materialized resource = %q, err=%v", materialized, readErr)
	}
	expression, err := bindRenderResourcesExpression(resourceURLs)
	if err != nil || !strings.Contains(expression, "media-0") || !strings.Contains(expression, "data-render-resource") {
		cleanup()
		t.Fatalf("binding expression = %q, err=%v", expression, err)
	}
	cleanup()
	if _, statErr := os.Stat(documentPath); !os.IsNotExist(statErr) {
		t.Fatalf("render document was not removed: %v", statErr)
	}
	if _, statErr := os.Stat(resourcePath); !os.IsNotExist(statErr) {
		t.Fatalf("render resource was not removed: %v", statErr)
	}
}

func windowsFileURLPath(value *url.URL) string {
	path := filepath.FromSlash(value.Path)
	if len(path) >= 3 && path[0] == filepath.Separator && path[2] == ':' {
		path = path[1:]
	}
	return path
}
