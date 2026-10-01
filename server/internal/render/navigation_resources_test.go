package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrefetchedResourceDoesNotWaitForOriginalImage(t *testing.T) {
	server, allowedPort := newTestAssetServer(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	var encoded bytes.Buffer
	green := color.RGBA{R: 16, G: 200, B: 80, A: 255}
	if err := png.Encode(&encoded, singlePixel(green)); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "green.png")
	if err := os.WriteFile(file, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded.Bytes())
	runner := newTestChromiumRunner(t, allowedPort)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	content, err := runner.Render(ctx, Document{Width: 64, Height: 64, Output: "png",
		HTML:      fmt.Sprintf(`<html><body style="margin:0"><img style="width:64px;height:64px" src="%s/stalled.png" data-render-resource="cover"></body></html>`, server.URL),
		Resources: []RenderResource{{ID: "cover", Path: file, MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(encoded.Len())}},
	})
	if err != nil {
		t.Fatal(err)
	}
	shot, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(shot.At(32, 32)); got != green {
		t.Fatalf("prefetched image not painted: %v", got)
	}
}

func TestRenderFailurePreservesPhaseAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := NewChromiumRunner(ChromiumOptions{TempRoot: t.TempDir()})
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Error(err)
		}
	})
	_, err := runner.Render(ctx, Document{})
	var staged interface{ RenderingPhase() string }
	if !errors.Is(err, context.Canceled) || !errors.As(err, &staged) || staged.RenderingPhase() != "browser_startup" {
		t.Fatalf("render phase or cancellation was lost: %v", err)
	}
}
