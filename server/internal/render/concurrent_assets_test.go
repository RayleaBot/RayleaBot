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

func TestChromiumRunnerConcurrentTabsWaitIndependentlyForAssets(t *testing.T) {
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	font, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "help.menu", "assets", "fonts", "noto-sans-sc", "k3kXo84MPvpLmixcA63oeALRLoKI.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	resourcePath := filepath.Join(t.TempDir(), "invalid.png")
	if err := os.WriteFile(resourcePath, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(invalid)
	var initial bytes.Buffer
	if err := png.Encode(&initial, singlePixel(color.RGBA{R: 240, G: 16, B: 16, A: 255})); err != nil {
		t.Fatal(err)
	}
	initialURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(initial.Bytes())
	colors := []color.RGBA{{R: 16, G: 80, B: 240, A: 255}, {R: 240, G: 160, B: 16, A: 255}}
	type gate struct {
		font, image, release chan struct{}
		once                 sync.Once
	}
	gates := []*gate{{font: make(chan struct{}), image: make(chan struct{}), release: make(chan struct{})}, {font: make(chan struct{}), image: make(chan struct{}), release: make(chan struct{})}}
	mux := http.NewServeMux()
	for index, g := range gates {
		var picture bytes.Buffer
		if err := png.Encode(&picture, singlePixel(colors[index])); err != nil {
			t.Fatal(err)
		}
		for _, asset := range []struct {
			name, contentType string
			payload           []byte
			requested         chan struct{}
		}{
			{"font", "font/woff2", font, g.font}, {"image", "image/png", picture.Bytes(), g.image},
		} {
			var requestedOnce sync.Once
			mux.HandleFunc(fmt.Sprintf("/%d/%s", index, asset.name), func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Content-Type", asset.contentType)
				w.Header().Set("Cache-Control", "no-store")
				requestedOnce.Do(func() { close(asset.requested) })
				select {
				case <-g.release:
					_, _ = w.Write(asset.payload)
				case <-r.Context().Done():
				}
			})
		}
	}
	endpoint := httptest.NewServer(mux)
	t.Cleanup(endpoint.Close)
	for _, g := range gates {
		t.Cleanup(func() { g.once.Do(func() { close(g.release) }) })
	}
	type result struct {
		image []byte
		err   error
	}
	completed := []chan result{make(chan result, 1), make(chan result, 1)}
	for index, g := range gates {
		doc := Document{Width: 64, Height: 64, Output: "png", HTML: fmt.Sprintf(`<!doctype html><html>
<head><style>body {margin:0;width:64px;height:64px;background:rgb(240,16,16)} img {display:block;width:32px;height:64px}</style></head>
<body><img src="%s" data-fallback="%s/%d/image" data-render-resource="media-0"><script>
window.addEventListener("load",()=>{const face=new FontFace("DelayedProbe",'url("%s/%d/font")');document.fonts.add(face);document.body.style.fontFamily="DelayedProbe";face.load().then(()=>{document.body.style.backgroundColor="rgb(16,240,16)";});});
</script></body></html>`, initialURL, endpoint.URL, index, endpoint.URL, index),
			Resources: []RenderResource{{ID: "media-0", Path: resourcePath, MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(invalid))}},
		}
		go func() { image, err := runner.Render(ctx, doc); completed[index] <- result{image, err} }()
		for _, requested := range []<-chan struct{}{g.font, g.image} {
			select {
			case <-requested:
			case got := <-completed[index]:
				t.Fatalf("render %d completed before both resources arrived: %v", index, got.err)
			case <-ctx.Done():
				t.Fatalf("render %d did not request its assets", index)
			}
		}
	}
	// Both tabs now have pending real font and image responses. Completing the
	// first must not depend on closing or foregrounding the still-blocked second.
	for index, g := range gates {
		select {
		case got := <-completed[index]:
			t.Fatalf("render %d completed before release: %v", index, got.err)
		default:
		}
		g.once.Do(func() { close(g.release) })
		var got result
		select {
		case got = <-completed[index]:
		case <-ctx.Done():
			t.Fatalf("render %d stalled behind another tab", index)
		}
		if got.err != nil {
			t.Fatalf("render %d: %v", index, got.err)
		}
		picture, err := png.Decode(bytes.NewReader(got.image))
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != 64 || picture.Bounds().Dy() != 64 {
			t.Fatalf("render %d bounds: %v", index, picture.Bounds())
		}
		for _, pixel := range []struct {
			x    int
			want color.RGBA
		}{{16, colors[index]}, {48, color.RGBA{R: 16, G: 240, B: 16, A: 255}}} {
			r, g, b, a := picture.At(pixel.x, 32).RGBA()
			actual := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
			if actual != pixel.want {
				t.Fatalf("render %d pixel %d = %v want %v", index, pixel.x, actual, pixel.want)
			}
		}
	}
}
