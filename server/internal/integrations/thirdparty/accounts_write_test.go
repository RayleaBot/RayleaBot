package thirdparty

import (
	"context"
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
