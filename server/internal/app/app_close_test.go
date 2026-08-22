package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

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
