package common

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

func TestServicePollPersistsSucceededQRCodeLogin(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	accounts := &stubAccountStore{}
	service := NewService(map[string]Provider{
		thirdparty.PlatformWeibo: stubProvider{
			create: LoginSession{
				Platform:  thirdparty.PlatformWeibo,
				Token:     "token",
				QRCodeURL: "https://example.test/qr",
				ExpiresAt: now.Add(3 * time.Minute),
				State:     StatePendingScan,
			},
			poll: LoginSession{
				State:  StateSucceeded,
				Cookie: "SUB=fixture; SUBP=fixture;",
				Account: thirdparty.AccountProfile{
					UID:       "123456",
					Nickname:  "微博扫码账号",
					AvatarURL: "https://example.test/avatar.jpg",
				},
			},
		},
	}, func() time.Time { return now }, WithAccountStore(accounts))

	created, err := service.Create(context.Background(), thirdparty.PlatformWeibo)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	polled, err := service.Poll(context.Background(), thirdparty.PlatformWeibo, created.LoginID)
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}

	if len(accounts.requests) != 1 {
		t.Fatalf("saved requests = %d, want 1", len(accounts.requests))
	}
	request := accounts.requests[0]
	if request.Cookie != "SUB=fixture; SUBP=fixture;" || request.AccountID != "123456" || request.Credential.State != thirdparty.CredentialValid {
		t.Fatalf("unexpected saved request: %#v", request)
	}
	if polled.SavedAccount == nil || polled.SavedAccount.AccountID != "123456" {
		t.Fatalf("poll result missing saved account: %#v", polled.SavedAccount)
	}
}

func TestServiceCreateUsesProviderLoginIDPrefix(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	service := NewService(map[string]Provider{
		thirdparty.PlatformBilibili: prefixedProvider{
			stubProvider: stubProvider{
				create: LoginSession{
					Platform:  thirdparty.PlatformBilibili,
					Token:     "token",
					QRCodeURL: "https://example.test/qr",
					ExpiresAt: now.Add(3 * time.Minute),
					State:     StatePendingScan,
				},
			},
			prefix: "qr",
		},
	}, func() time.Time { return now })

	created, err := service.Create(context.Background(), thirdparty.PlatformBilibili)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(created.LoginID, "qr_") {
		t.Fatalf("login_id = %q, want qr_ prefix", created.LoginID)
	}
}

type stubProvider struct {
	create LoginSession
	poll   LoginSession
}

func (p stubProvider) Create(context.Context, time.Time) (LoginSession, error) {
	return p.create, nil
}

func (p stubProvider) Poll(context.Context, LoginSession, time.Time) (LoginSession, error) {
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
	requests []thirdparty.UpsertRequest
}

func (s *stubAccountStore) Upsert(_ context.Context, request thirdparty.UpsertRequest) (thirdparty.Account, error) {
	s.requests = append(s.requests, request)
	return thirdparty.Account{
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
