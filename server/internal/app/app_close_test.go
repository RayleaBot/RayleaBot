package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
)

func TestNewReleasesConfigLifecycleLockAfterBuildFailure(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config-as-directory")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatalf("create invalid config directory: %v", err)
	}
	if _, err := New(Options{ConfigPath: configPath}); err == nil {
		t.Fatal("New should fail when the config path is a directory")
	}
	lockPath, err := runtimepaths.ResolveConfigLifecycleLockPath(configPath)
	if err != nil {
		t.Fatalf("resolve config lifecycle lock: %v", err)
	}
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquire lock after failed build: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("close lock: %v", err)
	}
}

func TestAppCloseReleasesConfigLifecycleLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "user.yaml.runtime.lock")
	lock, err := filelock.Acquire(path)
	if err != nil {
		t.Fatalf("acquire config lifecycle lock: %v", err)
	}
	application := &App{configLifecycleLock: lock}
	if _, err := filelock.Acquire(path); !errors.Is(err, filelock.ErrLocked) {
		t.Fatalf("overlapping lock error = %v, want ErrLocked", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("close app: %v", err)
	}
	reacquired, err := filelock.Acquire(path)
	if err != nil {
		t.Fatalf("reacquire released config lifecycle lock: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("close reacquired lock: %v", err)
	}
}

func TestAppCloseReleasesThirdPartyQRCodeLoginSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &appCloseQRProvider{session: thirdparty.QRLoginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     "fixture-token",
		QRCodeURL: "https://example.test/qr",
		ExpiresAt: now.Add(3 * time.Minute),
		State:     thirdparty.QRLoginStatePendingScan,
	}}
	service := thirdparty.NewQRLoginService(
		map[string]thirdparty.QRLoginProvider{thirdparty.PlatformDouyin: provider},
		func() time.Time { return now },
	)
	if _, err := service.Create(context.Background(), thirdparty.PlatformDouyin); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	application := &App{services: Services{ThirdPartyQRLogin: service}}

	if err := application.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	provider.mu.Lock()
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("provider close calls = %d, want 1", closeCalls)
	}
}

type appCloseQRProvider struct {
	session thirdparty.QRLoginSession

	mu         sync.Mutex
	closeCalls int
}

func (p *appCloseQRProvider) Create(context.Context, time.Time) (thirdparty.QRLoginSession, error) {
	return p.session, nil
}

func (p *appCloseQRProvider) Poll(context.Context, thirdparty.QRLoginSession, time.Time) (thirdparty.QRLoginSession, error) {
	return p.session, nil
}

func (p *appCloseQRProvider) Close(thirdparty.QRLoginSession) {
	p.mu.Lock()
	p.closeCalls++
	p.mu.Unlock()
}
