package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServiceRenderRequestsAdaptiveDocumentHeight(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "help.menu")

	runner := &fakeRunner{}
	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-output"),
		Store:              openRenderTestStore(t),
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatalf("expected render document")
	}
	if !doc.AutoHeight {
		t.Fatalf("expected render document to request adaptive height")
	}
	if doc.Width != 960 || doc.Height != 640 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	if doc.BaseURL == "" || !strings.HasPrefix(doc.BaseURL, "file:") || !strings.HasSuffix(doc.BaseURL, "/templates/help.menu/") {
		t.Fatalf("unexpected template base URL: %q", doc.BaseURL)
	}
}

func TestServiceRenderHelpMenuUsesCompactPrefixesAndArgumentKinds(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	runner := &fakeRunner{}
	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         filepath.Join(t.TempDir(), "render-help-menu"),
		Store:              openRenderTestStore(t),
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title":            "订阅中心",
			"command_prefixes": []string{"/", "*", "~"},
			"groups": []map[string]any{{
				"title": "订阅操作",
				"items": []map[string]any{{
					"name":             "订阅b站推送",
					"description":      "订阅指定 Bilibili 用户的更新",
					"command_source":   "manifest",
					"command_prefixes": []string{"/", "*", "~"},
					"usage_args":       "[直播|视频|图文] UID或昵称",
					"usage_parts": []map[string]any{
						{"kind": "optional", "text": "直播|视频|图文"},
						{"kind": "required", "text": "UID或昵称"},
					},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected render document")
	}
	for _, want := range []string{
		`@import url("assets/fonts/noto-sans-sc/result.css");`,
		`--font-cjk: "Noto Sans SC"`,
		`class="command-prefixes"`,
		`class="command-prefixes__label">前缀</span>`,
		`class="command-prefix-cue" aria-label="需添加上方任一前缀">前缀</span>`,
		`class="command-argument command-argument--optional"`,
		`class="command-argument command-argument--required"`,
		">直播|视频|图文</span>",
		">UID或昵称</span>",
	} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("help menu html missing %q:\n%s", want, doc.HTML)
		}
	}
	for _, unwanted := range []string{"command-guide__block--prefixes", "command-usage__prefix"} {
		if strings.Contains(doc.HTML, unwanted) {
			t.Fatalf("help menu html contains obsolete per-command prefix markup %q:\n%s", unwanted, doc.HTML)
		}
	}
	usageStart := strings.Index(doc.HTML, `<div class="command-usage"`)
	if usageStart < 0 {
		t.Fatalf("help menu html missing command usage code:\n%s", doc.HTML)
	}
	codeOffset := strings.Index(doc.HTML[usageStart:], "<code>")
	if codeOffset < 0 {
		t.Fatalf("help menu html missing command usage code:\n%s", doc.HTML)
	}
	codeStart := usageStart + codeOffset + len("<code>")
	codeEndOffset := strings.Index(doc.HTML[codeStart:], "</code>")
	if codeEndOffset < 0 {
		t.Fatalf("help menu html missing command usage code:\n%s", doc.HTML)
	}
	codeEnd := codeStart + codeEndOffset
	if regexp.MustCompile(`>\s+<`).MatchString(doc.HTML[codeStart:codeEnd]) {
		t.Fatalf("help menu command usage contains layout whitespace:\n%s", doc.HTML[codeStart:codeEnd])
	}
}

func TestServiceRenderHelpMenuWritesUsageWithThePluginPrefix(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	service, err := NewService(Options{
		RepoRoot:           filepath.Join("..", "..", ".."),
		OutputRoot:         filepath.Join(t.TempDir(), "render-help-menu-prefixes"),
		Store:              openRenderTestStore(t),
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Data: map[string]any{
			"title":            "崩坏：星穹铁道",
			"command_prefixes": []string{"*", "星铁"},
			"prefix_chips": []map[string]any{
				{"text": "*", "dedicated": true},
				{"text": "/", "dedicated": false},
			},
			"groups": []map[string]any{{
				"title": "信息查询",
				"items": []map[string]any{{
					"name":             "体力",
					"description":      "查询开拓力",
					"trigger_type":     "exact",
					"command_prefixes": []string{"*", "/"},
					"primary_prefix":   "*",
					"usage_args":       "[UID]",
					"usage_parts":      []map[string]any{{"kind": "optional", "text": "UID"}},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatal("expected render document")
	}
	for _, want := range []string{
		`<code class="command-prefixes__dedicated" aria-label="专属前缀 *">*</code><code>/</code>`,
		`<code><span class="command-usage__lead">*</span><span class="command-usage__name">体力</span>`,
	} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("help menu html missing %q:\n%s", want, doc.HTML)
		}
	}
	if strings.Contains(doc.HTML, `class="command-prefix-cue"`) {
		t.Fatalf("a command with a known prefix still shows the placeholder cue:\n%s", doc.HTML)
	}
}

func TestServiceRenderLeaderboardListTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-leaderboard")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "leaderboard.list",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title":       "本周发言榜",
			"subtitle":    "统计周期：2026-05-01 至 2026-05-07",
			"value_label": "发言数",
			"items": []map[string]any{
				{
					"avatar_url":     "https://q.qlogo.cn/headimg_dl?dst_uin=10001&spec=640",
					"group_nickname": "测试群名片",
					"nickname":       "Silver",
					"title":          "群主",
					"value":          128,
				},
				{
					"nickname": "Nova",
					"value":    81,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc, ok := runner.lastDocument()
	if !ok {
		t.Fatalf("expected render document")
	}
	if !doc.AutoHeight {
		t.Fatalf("expected render document to request adaptive height")
	}
	if doc.Width != 960 || doc.Height != 420 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	for _, want := range []string{"测试群名片", "（Silver）", "群主", "Nova", "128", "81"} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("leaderboard html missing %q:\n%s", want, doc.HTML)
		}
	}
}

func TestServiceRenderRejectsInputTooLarge(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-limit")
	runner := &fakeRunner{}
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     1,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 32,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	_, err = service.Render(context.Background(), Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": strings.Repeat("x", 128),
		},
	})
	if err == nil {
		t.Fatal("expected oversized render data error")
	}

	var renderErr *Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if renderErr.Code != "platform.render_input_too_large" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.render_input_too_large")
	}
}

func TestChromiumRunnerLoadsRelativeTemplateAssets(t *testing.T) {
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	templatesRoot := filepath.Join(t.TempDir(), "templates")
	assetDir := filepath.Join(templatesRoot, "asset.check", "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("create asset dir: %v", err)
	}
	asset, err := os.Create(filepath.Join(assetDir, "red.png"))
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := png.Encode(asset, singlePixel(color.RGBA{R: 240, G: 16, B: 16, A: 255})); err != nil {
		_ = asset.Close()
		t.Fatalf("encode asset: %v", err)
	}
	if err := asset.Close(); err != nil {
		t.Fatalf("close asset: %v", err)
	}

	content, err := runner.Render(ctx, Document{
		Template:   "relative.asset",
		Output:     "png",
		BaseURL:    BaseURL(filepath.Join(templatesRoot, "asset.check")),
		Width:      320,
		Height:     240,
		AutoHeight: true,
		HTML: `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <style>
      :root {
        --asset-smoke: url("assets/red.png");
      }
      body { margin: 0; }
      main {
        width: 320px;
        height: 240px;
        background: #ffffff var(--asset-smoke) center / cover no-repeat;
      }
    </style>
  </head>
  <body><main aria-label="relative asset smoke"></main></body>
</html>`,
	})
	if err != nil {
		t.Fatalf("Render with relative asset: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected screenshot content")
	}

	screenshot, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	r, g, b, _ := screenshot.At(160, 120).RGBA()
	if r>>8 < 220 || g>>8 > 40 || b>>8 > 40 {
		t.Fatalf("relative asset did not paint expected pixel: got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestChromiumRunnerMeasuresPageFromBodyThatHidesOverflow(t *testing.T) {
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	for _, test := range []struct {
		overflow string
		height   int
	}{{"visible", 280}, {"hidden", 100}} {
		content, err := runner.Render(ctx, Document{
			Template: "overflow.check", Output: "png", Width: 320, Height: 60, AutoHeight: true,
			HTML: `<!doctype html><html><head><style>
      body { margin: 0; width: 320px; height: 100px; position: relative; overflow: ` + test.overflow + `; }
      div { position: absolute; top: 80px; width: 40px; height: 200px; background: #f00; }
    </style></head><body><div></div></body></html>`,
		})
		if err != nil {
			t.Fatalf("Render with overflow %s: %v", test.overflow, err)
		}
		screenshot, err := png.Decode(bytes.NewReader(content))
		if err != nil {
			t.Fatalf("decode screenshot: %v", err)
		}
		if screenshot.Bounds().Dy() != test.height {
			t.Errorf("overflow %s: height = %d, want %d", test.overflow, screenshot.Bounds().Dy(), test.height)
		}
	}
}

func TestChromiumRunnerFitsWidthToBody(t *testing.T) {
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	for _, test := range []struct {
		fit   bool
		width int
	}{{false, 400}, {true, 250}} {
		content, err := runner.Render(ctx, Document{
			Template: "width.check", Output: "png", Width: 400, Height: 60, FitWidth: test.fit, AutoHeight: true,
			HTML: `<!doctype html><html><head><style>
      body { margin: 0; width: 250px; height: 100px; background: #0f0; }
    </style></head><body></body></html>`,
		})
		if err != nil {
			t.Fatalf("Render with fit %v: %v", test.fit, err)
		}
		screenshot, err := png.Decode(bytes.NewReader(content))
		if err != nil {
			t.Fatalf("decode screenshot: %v", err)
		}
		if screenshot.Bounds().Dx() != test.width {
			t.Errorf("fit %v: width = %d, want %d", test.fit, screenshot.Bounds().Dx(), test.width)
		}
	}
}

func TestChromiumRunnerRestoresSourceWhenPrefetchedResourceCannotDecode(t *testing.T) {
	runner := newTestChromiumRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	invalidContent := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	resourcePath := filepath.Join(t.TempDir(), "invalid.png")
	if err := os.WriteFile(resourcePath, invalidContent, 0o600); err != nil {
		t.Fatalf("write invalid resource: %v", err)
	}
	digest := sha256.Sum256(invalidContent)

	var fallback bytes.Buffer
	if err := png.Encode(&fallback, singlePixel(color.RGBA{R: 16, G: 80, B: 240, A: 255})); err != nil {
		t.Fatalf("encode fallback: %v", err)
	}
	fallbackURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(fallback.Bytes())

	content, err := runner.Render(ctx, Document{
		Template: "prefetched.resource.fallback",
		Output:   "png",
		Width:    64,
		Height:   64,
		HTML: `<!doctype html>
<html lang="zh-CN">
  <head><meta charset="utf-8" /><style>body { margin: 0; } img { width: 64px; height: 64px; display: block; }</style></head>
  <body><img src="` + fallbackURL + `" data-render-resource="media-0" alt="" /></body>
</html>`,
		Resources: []RenderResource{{
			ID: "media-0", Path: resourcePath, MIME: "image/png", SHA256: hex.EncodeToString(digest[:]), Size: int64(len(invalidContent)),
		}},
	})
	if err != nil {
		t.Fatalf("Render with invalid prefetched resource: %v", err)
	}
	screenshot, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	r, g, b, _ := screenshot.At(32, 32).RGBA()
	if r>>8 > 40 || g>>8 < 60 || g>>8 > 100 || b>>8 < 220 {
		t.Fatalf("source fallback did not paint expected pixel: got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestServiceRenderRejectsQueueFull(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-queue")
	waitCh := make(chan struct{})
	runner := &fakeRunner{waitCh: waitCh}
	var closeWait sync.Once
	store := openRenderTestStore(t)

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     1,
		QueueWaitTimeout:   5 * time.Second,
		RenderTimeout:      5 * time.Second,
		MaxRenderDataBytes: 256 * 1024,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() {
		closeWait.Do(func() {
			close(waitCh)
		})
		if err := service.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
		},
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Render(context.Background(), request)
		firstDone <- err
	}()

	secondDone := make(chan error, 1)
	go func() {
		_, err := service.Render(context.Background(), request)
		secondDone <- err
	}()

	waitForActiveRenders(t, service.worker, 2, 2*time.Second)

	_, err = service.Render(context.Background(), request)
	if err == nil {
		t.Fatal("expected queue full error")
	}

	var renderErr *Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if renderErr.Code != "platform.render_queue_full" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.render_queue_full")
	}

	closeWait.Do(func() {
		close(waitCh)
	})
	if err := <-firstDone; err != nil {
		t.Fatalf("first render failed after release: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second render failed after release: %v", err)
	}
}

// waitForActiveRenders polls the worker's admission count so the test knows
// both earlier requests hold a slot before submitting the overflow request.
func waitForActiveRenders(t *testing.T, worker *Worker, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		worker.mu.Lock()
		active := worker.activeRequests
		worker.mu.Unlock()
		if active >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d active renders, got %d", want, active)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
