package thirdparty

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestServicePollPersistsSucceededQRCodeLogin(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	accounts := &stubAccountStore{}
	service := NewQRLoginService(map[string]QRLoginProvider{
		PlatformWeibo: stubProvider{
			create: QRLoginSession{
				Platform:  PlatformWeibo,
				Token:     "token",
				QRCodeURL: "https://example.test/qr",
				ExpiresAt: now.Add(3 * time.Minute),
				State:     QRLoginStatePendingScan,
			},
			poll: QRLoginSession{
				State:  QRLoginStateSucceeded,
				Cookie: "SUB=fixture; SUBP=fixture;",
				Account: AccountProfile{
					UID:       "123456",
					Nickname:  "微博扫码账号",
					AvatarURL: "https://example.test/avatar.jpg",
				},
			},
		},
	}, func() time.Time { return now }, WithQRLoginAccountStore(accounts))

	created, err := service.Create(context.Background(), PlatformWeibo)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	polled, err := service.Poll(context.Background(), PlatformWeibo, created.LoginID)
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}

	if len(accounts.requests) != 1 {
		t.Fatalf("saved requests = %d, want 1", len(accounts.requests))
	}
	request := accounts.requests[0]
	if request.Cookie != "SUB=fixture; SUBP=fixture;" || request.AccountID != "123456" || request.Credential.State != CredentialValid {
		t.Fatalf("unexpected saved request: %#v", request)
	}
	if polled.SavedAccount == nil || polled.SavedAccount.AccountID != "123456" {
		t.Fatalf("poll result missing saved account: %#v", polled.SavedAccount)
	}
}

func TestServiceCreateUsesProviderLoginIDPrefix(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewQRLoginService(map[string]QRLoginProvider{
		PlatformBilibili: prefixedProvider{
			stubProvider: stubProvider{
				create: QRLoginSession{
					Platform:  PlatformBilibili,
					Token:     "token",
					QRCodeURL: "https://example.test/qr",
					ExpiresAt: now.Add(3 * time.Minute),
					State:     QRLoginStatePendingScan,
				},
			},
			prefix: "qr",
		},
	}, func() time.Time { return now })

	created, err := service.Create(context.Background(), PlatformBilibili)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(created.LoginID, "qr_") {
		t.Fatalf("login_id = %q, want qr_ prefix", created.LoginID)
	}
}

type stubProvider struct {
	create QRLoginSession
	poll   QRLoginSession
}

func (p stubProvider) Create(context.Context, time.Time) (QRLoginSession, error) {
	return p.create, nil
}

func (p stubProvider) Poll(context.Context, QRLoginSession, time.Time) (QRLoginSession, error) {
	return p.poll, nil
}

type prefixedProvider struct {
	stubProvider
	prefix string
}

func (p prefixedProvider) LoginIDPrefix() string {
	return p.prefix
}

type stubAccountStore struct {
	requests []UpsertRequest
}

func (s *stubAccountStore) Upsert(_ context.Context, request UpsertRequest) (Account, error) {
	s.requests = append(s.requests, request)
	return Account{
		Platform:   request.Platform,
		AccountID:  request.AccountID,
		Label:      request.Label,
		Enabled:    request.Enabled,
		Configured: request.Cookie != "",
		Profile:    request.Profile,
		Credential: request.Credential,
		UpdatedAt:  time.Date(2026, 6, 8, 8, 0, 1, 0, time.UTC),
	}, nil
}
