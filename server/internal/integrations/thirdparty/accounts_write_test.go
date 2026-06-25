package thirdparty

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestUpsertPreservesRequestProfileWhenValidatorReturnsEmptyProfile(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	checkedAt := time.Date(2026, 6, 8, 8, 1, 1, 0, time.UTC)

	account, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformWeibo,
		AccountID: "primary",
		Label:     "微博主账号",
		Enabled:   true,
		Cookie:    "SUB=fixture;",
		Profile: AccountProfile{
			UID:       "123456",
			Nickname:  "微博扫码账号",
			AvatarURL: "https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg",
		},
		Validate: func(context.Context, string) (AccountProfile, CredentialStatus, error) {
			return AccountProfile{}, CredentialStatus{
				State:     CredentialUnknown,
				CheckedAt: &checkedAt,
				LastError: "weibo profile unavailable",
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if account.Profile.UID != "123456" || account.Profile.Nickname != "微博扫码账号" || account.Profile.AvatarURL == "" {
		t.Fatalf("unexpected saved profile: %#v", account.Profile)
	}
	if !account.Configured || account.Credential.State != CredentialUnknown || account.Credential.CheckedAt == nil {
		t.Fatalf("unexpected saved account state: %#v", account)
	}
}

func TestUpsertPersistsProxyConfigAcrossListPaths(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	proxyURL := "http://user:pass@127.0.0.1:8080"
	proxyEnabled := true

	account, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:     PlatformWeibo,
		AccountID:    "primary",
		Label:        "微博主账号",
		Enabled:      true,
		Cookie:       "SUB=fixture;",
		ProxyURL:     &proxyURL,
		ProxyEnabled: &proxyEnabled,
	})
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if account.ProxyURL != proxyURL || !account.ProxyEnabled {
		t.Fatalf("unexpected saved proxy config: %#v", account)
	}

	accounts, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ProxyURL != proxyURL || !accounts[0].ProxyEnabled {
		t.Fatalf("unexpected listed proxy config: %#v", accounts)
	}

	enabledAccounts, err := service.ListEnabled(context.Background(), PlatformWeibo)
	if err != nil {
		t.Fatalf("ListEnabled returned error: %v", err)
	}
	if len(enabledAccounts) != 1 || enabledAccounts[0].ProxyURL != proxyURL || !enabledAccounts[0].ProxyEnabled {
		t.Fatalf("unexpected enabled proxy config: %#v", enabledAccounts)
	}
}

func TestUpsertPreservesProxyConfigWhenRequestOmitsProxyFields(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	proxyURL := "socks5://127.0.0.1:1080"
	proxyEnabled := true
	if _, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:     PlatformDouyin,
		AccountID:    "primary",
		Label:        "抖音主账号",
		Enabled:      true,
		Cookie:       "sessionid=fixture;",
		ProxyURL:     &proxyURL,
		ProxyEnabled: &proxyEnabled,
	}); err != nil {
		t.Fatalf("seed Upsert returned error: %v", err)
	}

	account, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformDouyin,
		AccountID: "primary",
		Label:     "抖音主账号",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}
	if account.ProxyURL != proxyURL || !account.ProxyEnabled {
		t.Fatalf("proxy config was not preserved: %#v", account)
	}
}

func TestUpsertRejectsEnabledProxyWithoutURL(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	proxyURL := ""
	proxyEnabled := true

	_, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:     PlatformWeibo,
		AccountID:    "primary",
		Label:        "微博主账号",
		Enabled:      true,
		ProxyURL:     &proxyURL,
		ProxyEnabled: &proxyEnabled,
	})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("Upsert error = %v, want ErrInvalidAccount", err)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("secrets.NewSQLiteStore: %v", err)
	}
	service, err := NewService(store, secretStore)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}
