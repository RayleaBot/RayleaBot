package service

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
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

func TestServiceRenderLeaderboardListTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
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

func TestServiceRenderFortuneCardTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-fortune")
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
		Template: "fortune.card",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title":         "今日运势",
			"subtitle":      "2026-05-04",
			"repeat_notice": "今日运势已经抽取过，以下为当日结果。",
			"user": map[string]any{
				"group_nickname": "测试群名片",
				"nickname":       "Silver",
				"title":          "群主",
				"id":             "10001",
			},
			"group": map[string]any{
				"name": "测试群",
			},
			"fortune": map[string]any{
				"name":        "大吉",
				"stars":       "★★★★★★★",
				"sign":        "云开见月，万事顺遂",
				"explanation": "适合推进重要事项。",
			},
			"today_good": []string{"整理计划", "主动沟通"},
			"today_bad":  []string{"熬夜", "拖延决定"},
			"streak": map[string]any{
				"current": 7,
				"total":   12,
			},
			"stats": []map[string]any{
				{"label": "累计大吉", "value": "2 次"},
				{"label": "最长连续大凶", "value": "1 天"},
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
	if doc.Width != 1124 || doc.Height != 1365 {
		t.Fatalf("unexpected initial render dimensions: got %dx%d", doc.Width, doc.Height)
	}
	if doc.BaseURL == "" || !strings.HasPrefix(doc.BaseURL, "file:") || !strings.HasSuffix(doc.BaseURL, "/templates/fortune.card/") {
		t.Fatalf("unexpected template base URL: %q", doc.BaseURL)
	}
	for _, want := range []string{"今日运势", "今日运势已经抽取过", "测试群名片", "群主", "大吉", "★★★★★★★", "连续签到"} {
		if !strings.Contains(doc.HTML, want) {
			t.Fatalf("fortune html missing %q:\n%s", want, doc.HTML)
		}
	}
	for _, unwanted := range []string{"累计大吉", "最长连续大凶", "运势统计"} {
		if strings.Contains(doc.HTML, unwanted) {
			t.Fatalf("fortune html contains %q:\n%s", unwanted, doc.HTML)
		}
	}
}

func TestServiceRenderRejectsInputTooLarge(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
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

	var renderErr *rendertemplates.Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *rendertemplates.Error, got %T", err)
	}
	if renderErr.Code != "platform.render_input_too_large" {
		t.Fatalf("unexpected error code: got %q want %q", renderErr.Code, "platform.render_input_too_large")
	}
}

func TestChromiumRunnerLoadsRelativeTemplateAssets(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..", "..")
	browserPath, err := deps.NewManager(repoRoot).ResolvePreparedEntrypoint("chromium", "browser")
	if err != nil {
		t.Skipf("managed chromium is not prepared: %v", err)
	}

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

	runner := renderbrowser.NewChromiumRunner(renderbrowser.ChromiumOptions{BrowserPath: browserPath})
	content, err := runner.Render(context.Background(), renderbrowser.Document{
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

func TestServiceRenderRejectsQueueFull(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..", "..")
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
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      2 * time.Second,
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

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runner.callCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	_, err = service.Render(context.Background(), request)
	if err == nil {
		t.Fatal("expected queue full error")
	}

	var renderErr *rendertemplates.Error
	if !errors.As(err, &renderErr) {
		t.Fatalf("expected *rendertemplates.Error, got %T", err)
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
