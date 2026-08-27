package douyin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func TestDouyinUserAgentForProduct(t *testing.T) {
	t.Parallel()

	cases := []struct {
		goos    string
		product string
		want    string
	}{
		{
			goos:    "windows",
			product: "Chrome/152.0.7977.42",
			want:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.7977.42 Safari/537.36",
		},
		{
			goos:    "darwin",
			product: "Chrome/150.0.0.0",
			want:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
		},
		{
			goos:    "linux",
			product: "Chromium/149.0.0.0",
			want:    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chromium/149.0.0.0 Safari/537.36",
		},
	}
	for _, tc := range cases {
		if got := douyinUserAgentForProduct(tc.goos, tc.product); got != tc.want {
			t.Fatalf("douyinUserAgentForProduct(%q, %q) = %q, want %q", tc.goos, tc.product, got, tc.want)
		}
	}
	if got := douyinUserAgentForProduct("windows", "  "); got != "" {
		t.Fatalf("empty product must yield empty UA, got %q", got)
	}
}

// TestChromedpBrowserProfileSlotRejectsWhenBusy 持久化 profile 同一时间
// 只能被一个浏览器进程占用；被占用且剩余尝试都不使用 profile 时直接
// 拒绝本次登录，而不是回退到无 profile 的 headless（无设备信誉的登录
// 环境会触发新设备风控，且丢失既有登录态）。
func TestChromedpBrowserProfileSlotRejectsWhenBusy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	browser := NewChromedpBrowser(BrowserOptions{Mode: BrowserModeAuto, UserDataDir: t.TempDir()})
	// 模拟另一个登录会话占用 profile。
	<-browser.profileSlot

	var visited []string
	browser.attemptRunner = func(_ context.Context, attempt browserLaunchAttempt, _ time.Time) (BrowserCreateResult, *browserRuntime, error) {
		visited = append(visited, attempt.mode)
		return BrowserCreateResult{}, nil, context.DeadlineExceeded
	}
	_, err := browser.createWithAttempts(context.Background(), now, []browserLaunchAttempt{
		{mode: BrowserModeVisible, useProfile: true},
		{mode: BrowserModeHeadless},
	})
	if !errors.Is(err, thirdparty.ErrQRLoginBrowserBusy) {
		t.Fatalf("createWithAttempts() error = %v, want ErrQRLoginBrowserBusy", err)
	}
	if len(visited) != 0 {
		t.Fatalf("attempt modes = %v, want no attempts while profile busy", visited)
	}
}

// TestChromedpBrowserProfileSlotReleasedOnClose 使用持久化 profile 的会话关闭后，
// profile 使用权必须归还，后续登录可以再次使用可见模式。
func TestChromedpBrowserProfileSlotReleasedOnClose(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	browser := NewChromedpBrowser(BrowserOptions{Mode: BrowserModeVisible, UserDataDir: t.TempDir()})
	browser.attemptRunner = func(_ context.Context, attempt browserLaunchAttempt, _ time.Time) (BrowserCreateResult, *browserRuntime, error) {
		return BrowserCreateResult{
				Token:     "fixture-token",
				QRCodeURL: "https://example.test/fixture-qr",
				ExpiresAt: now.Add(time.Minute),
			}, &browserRuntime{
				ctx:       context.Background(),
				cancel:    func() {},
				expiresAt: now.Add(time.Minute),
				mode:      attempt.mode,
				capture:   &douyinNetworkCapture{},
				cookies:   map[string]string{},
				lastState: "pending_scan",
			}, nil
	}
	result, err := browser.createWithAttempts(context.Background(), now, []browserLaunchAttempt{{mode: BrowserModeVisible, useProfile: true}})
	if err != nil {
		t.Fatalf("createWithAttempts() error = %v", err)
	}
	if runtime := browser.session(result.Token); runtime == nil || !runtime.usesProfile {
		t.Fatal("visible attempt with profile dir must mark runtime as profile user")
	}
	browser.Close(result.Token)

	select {
	case <-browser.profileSlot:
	case <-time.After(3 * time.Second):
		t.Fatal("profile slot was not released after session close")
	}
}

// TestChromedpBrowserWithoutProfileDirKeepsConcurrentVisible 未配置持久化目录时
// 不应启用信号量，多个可见会话互不阻塞（保持既有行为）。
func TestChromedpBrowserWithoutProfileDirKeepsConcurrentVisible(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	browser := NewChromedpBrowser(BrowserOptions{Mode: BrowserModeVisible})
	if browser.profileSlot != nil {
		t.Fatal("profile slot must be nil when UserDataDir is empty")
	}
	browser.attemptRunner = func(_ context.Context, attempt browserLaunchAttempt, _ time.Time) (BrowserCreateResult, *browserRuntime, error) {
		return BrowserCreateResult{
				Token:     "fixture-token",
				QRCodeURL: "https://example.test/fixture-qr",
				ExpiresAt: now.Add(time.Minute),
			}, &browserRuntime{
				ctx:       context.Background(),
				cancel:    func() {},
				expiresAt: now.Add(time.Minute),
				mode:      attempt.mode,
				capture:   &douyinNetworkCapture{},
				cookies:   map[string]string{},
				lastState: "pending_scan",
			}, nil
	}
	result, err := browser.createWithAttempts(context.Background(), now, []browserLaunchAttempt{{mode: BrowserModeVisible}})
	if err != nil {
		t.Fatalf("createWithAttempts() error = %v", err)
	}
	defer browser.Close(result.Token)
}
