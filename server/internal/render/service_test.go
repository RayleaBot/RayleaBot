package render

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var testPNGBytes, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")

type fakeRunner struct {
	mu      sync.Mutex
	calls   int
	delay   time.Duration
	waitCh  chan struct{}
	content []byte
	err     error
}

func (f *fakeRunner) Render(ctx context.Context, doc Document) ([]byte, error) {
	f.mu.Lock()
	f.calls++
	delay := f.delay
	waitCh := f.waitCh
	content := append([]byte(nil), f.content...)
	err := f.err
	f.mu.Unlock()

	if waitCh != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitCh:
		}
	}

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		content = append([]byte(nil), testPNGBytes...)
	}
	if doc.Output == "jpeg" {
		return []byte{0xff, 0xd8, 0xff, 0xd9}, nil
	}
	return content, nil
}

func (f *fakeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestServiceRenderCachesArtifacts(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-cache")
	runner := &fakeRunner{}

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
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

	request := Request{
		Template: "help.menu",
		Theme:    "default",
		Output:   "png",
		Data: map[string]any{
			"title": "帮助菜单",
			"items": []map[string]any{
				{"name": "weather", "description": "查询天气", "usage": "/weather <城市>"},
			},
		},
	}

	first, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if first.FromCache {
		t.Fatalf("expected first render to miss cache")
	}
	if first.ArtifactID == "" || first.CacheKey == "" || first.ImagePath == "" {
		t.Fatalf("expected artifact metadata, got %#v", first)
	}

	second, err := service.Render(context.Background(), request)
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if !second.FromCache {
		t.Fatalf("expected second render to hit cache")
	}
	if second.ArtifactID != first.ArtifactID || second.CacheKey != first.CacheKey {
		t.Fatalf("expected stable cache result: first=%#v second=%#v", first, second)
	}
	if runner.callCount() != 1 {
		t.Fatalf("runner call count = %d, want 1", runner.callCount())
	}
}

func TestServiceRenderRejectsInputTooLarge(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-limit")
	runner := &fakeRunner{}

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
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

func TestServiceRenderRejectsQueueFull(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..", "..")
	outputRoot := filepath.Join(t.TempDir(), "render-queue")
	waitCh := make(chan struct{})
	runner := &fakeRunner{waitCh: waitCh}
	var closeWait sync.Once

	service, err := NewService(Options{
		RepoRoot:           repoRoot,
		OutputRoot:         outputRoot,
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
