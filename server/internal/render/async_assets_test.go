package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestChromiumRunnerWaitsForDelayedFont(t *testing.T) {
	font, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "help.menu", "assets", "fonts", "noto-sans-sc", "k3kXo84MPvpLmixcA63oeALRLoKI.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	assertDelayedResourcePaint(t, "font/woff2", font, color.RGBA{R: 16, G: 240, B: 16, A: 255}, func(endpoint string) Document {
		return Document{Width: 64, Height: 64, Output: "png", HTML: fmt.Sprintf(`<!doctype html>
<html><head><style>body {margin:0; width:64px; height:64px; background:rgb(240,16,16)}</style></head>
<body><script>
window.addEventListener("load", () => {
  const face = new FontFace("DelayedProbe", 'url("%s")');
  document.fonts.add(face);
  document.body.style.fontFamily = "DelayedProbe";
  face.load().then(() => { document.body.style.backgroundColor = "rgb(16,240,16)"; });
});
</script></body></html>`, endpoint)}
	})
}

func TestChromiumRunnerWaitsForDelayedImageFallback(t *testing.T) {
	invalid := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	resourcePath := filepath.Join(t.TempDir(), "invalid.png")
	if err := os.WriteFile(resourcePath, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(invalid)
	var initial, fallback bytes.Buffer
	if err := png.Encode(&initial, singlePixel(color.RGBA{R: 240, G: 16, B: 16, A: 255})); err != nil {
		t.Fatal(err)
	}
	blue := color.RGBA{R: 16, G: 80, B: 240, A: 255}
	if err := png.Encode(&fallback, singlePixel(blue)); err != nil {
		t.Fatal(err)
	}
	initialURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(initial.Bytes())
	assertDelayedResourcePaint(t, "image/png", fallback.Bytes(), blue, func(endpoint string) Document {
		return Document{Width: 64, Height: 64, Output: "png",
			HTML: fmt.Sprintf(`<!doctype html><html><head><style>body {margin:0} img {display:block;width:64px;height:64px}</style></head>
<body><img src="%s" data-fallback="%s" data-render-resource="media-0"></body></html>`, initialURL, endpoint),
			Resources: []RenderResource{{ID: "media-0", Path: resourcePath, MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(invalid))}},
		}
	})
}

func assertDelayedResourcePaint(t *testing.T, contentType string, payload []byte, want color.RGBA, document func(string) Document) {
	t.Helper()
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	requested, release := make(chan struct{}), make(chan struct{})
	var requestOnce, releaseOnce sync.Once
	releaseResponse := func() { releaseOnce.Do(func() { close(release) }) }
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", contentType)
		requestOnce.Do(func() { close(requested) })
		select {
		case <-release:
			_, _ = w.Write(payload)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(endpoint.Close)
	t.Cleanup(releaseResponse)
	type result struct {
		image []byte
		err   error
	}
	finished := make(chan result, 1)
	go func() {
		image, err := runner.Render(ctx, document(endpoint.URL+"/delayed"))
		finished <- result{image, err}
	}()
	select {
	case <-requested:
	case got := <-finished:
		t.Fatalf("render finished before requesting delayed resource: %v", got.err)
	case <-ctx.Done():
		t.Fatal("browser never requested delayed resource")
	}
	// The network response remains withheld. A successful screenshot here is
	// observable premature output, regardless of how the JS promises are built.
	select {
	case got := <-finished:
		t.Fatalf("render completed while its resource response was blocked: err=%v PNG bytes=%d", got.err, len(got.image))
	case <-time.After(300 * time.Millisecond):
	}
	releaseResponse()
	var got result
	select {
	case got = <-finished:
	case <-ctx.Done():
		t.Fatal("render did not complete after the resource was released")
	}
	if got.err != nil {
		t.Fatalf("render delayed resource: %v", got.err)
	}
	screenshot, err := png.Decode(bytes.NewReader(got.image))
	if err != nil {
		t.Fatal(err)
	}
	if screenshot.Bounds().Dx() != 64 || screenshot.Bounds().Dy() != 64 {
		t.Fatalf("unexpected bounds: %v", screenshot.Bounds())
	}
	r, g, b, a := screenshot.At(32, 32).RGBA()
	actual := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if actual != want {
		t.Fatalf("resource was not painted: got %v want %v", actual, want)
	}
}
