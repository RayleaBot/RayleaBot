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

func TestUpsertKeepsTransientValidationFailureUnknown(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	checkedAt := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	account, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformWeibo,
		AccountID: "primary",
		Label:     "微博主账号",
		Enabled:   true,
		Cookie:    "SUB=fixture;",
		Validate: func(context.Context, string) (AccountProfile, CredentialStatus, error) {
			return AccountProfile{}, CredentialStatus{
				State:     CredentialUnknown,
				CheckedAt: &checkedAt,
				LastError: "微博 CK 状态暂时无法确认，请稍后重试",
			}, errors.New("upstream unavailable")
		},
	})
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if account.Credential.State != CredentialUnknown || account.Credential.LastError == "" {
		t.Fatalf("transient validation was not preserved as unknown: %#v", account.Credential)
	}
}

func TestUpsertPreservesStoredProfileWhenReplacementValidationHasNoProfile(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	initialCheckedAt := time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)
	_, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformNeteaseMusic,
		AccountID: "primary",
		Label:     "网易云音乐主账号",
		Enabled:   true,
		Cookie:    "MUSIC_U=fixture-old;",
		Profile: AccountProfile{
			UID:       "573268415",
			Nickname:  "bronyaiu",
			AvatarURL: "https://p4.music.126.net/fixture/avatar.jpg",
		},
		Credential: CredentialStatus{State: CredentialValid, CheckedAt: &initialCheckedAt},
	})
	if err != nil {
		t.Fatalf("save initial account: %v", err)
	}

	replacementCheckedAt := initialCheckedAt.Add(time.Hour)
	account, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformNeteaseMusic,
		AccountID: "primary",
		Label:     "网易云音乐主账号",
		Enabled:   true,
		Cookie:    "MUSIC_U=fixture-new;",
		Validate: func(context.Context, string) (AccountProfile, CredentialStatus, error) {
			return AccountProfile{}, CredentialStatus{
				State:     CredentialUnknown,
				CheckedAt: &replacementCheckedAt,
				LastError: "网易云音乐 CK 状态暂时无法确认，请稍后重试",
			}, errors.New("upstream unavailable")
		},
	})
	if err != nil {
		t.Fatalf("save replacement account: %v", err)
	}
	if account.Profile.UID != "573268415" || account.Profile.Nickname != "bronyaiu" || account.Profile.AvatarURL != "https://p4.music.126.net/fixture/avatar.jpg" {
		t.Fatalf("stored profile was not preserved: %#v", account.Profile)
	}
	if account.Credential.State != CredentialUnknown || account.Credential.CheckedAt == nil || !account.Credential.CheckedAt.Equal(replacementCheckedAt) {
		t.Fatalf("replacement credential status = %#v", account.Credential)
	}
}

func TestCredentialStatusCompareAndSwapPreservesNewCredential(t *testing.T) {
	t.Parallel()

	service := newTestService(t)
	firstTime := time.Date(2026, 8, 21, 1, 0, 0, 100, time.UTC)
	service.now = func() time.Time { return firstTime }
	stale, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformWeibo,
		AccountID: "primary",
		Label:     "微博主账号",
		Enabled:   true,
		Cookie:    "SUB=old;",
		Profile:   AccountProfile{UID: "123456", Nickname: "旧资料"},
		Credential: CredentialStatus{
			State:     CredentialValid,
			CheckedAt: &firstTime,
		},
	})
	if err != nil {
		t.Fatalf("save initial account: %v", err)
	}

	secondTime := firstTime.Add(time.Nanosecond)
	service.now = func() time.Time { return secondTime }
	current, err := service.Upsert(context.Background(), UpsertRequest{
		Platform:  PlatformWeibo,
		AccountID: "primary",
		Label:     "微博主账号",
		Enabled:   true,
		Cookie:    "SUB=new;",
		Profile:   AccountProfile{UID: "654321", Nickname: "新资料"},
		Credential: CredentialStatus{
			State:     CredentialValid,
			CheckedAt: &secondTime,
		},
	})
	if err != nil {
		t.Fatalf("save replacement account: %v", err)
	}
	checkedAt := secondTime.Add(time.Minute)
	updated, applied, err := service.UpdateCredentialStatusIfUnchanged(context.Background(), stale, AccountProfile{}, CredentialStatus{
		State:     CredentialInvalid,
		CheckedAt: &checkedAt,
		LastError: "stale result",
	})
	if err != nil {
		t.Fatalf("apply stale credential result: %v", err)
	}
	if applied {
		t.Fatal("stale credential result overwrote a newer saved credential")
	}
	if updated.UpdatedAt != current.UpdatedAt || updated.Profile.UID != "654321" || updated.Profile.Nickname != "新资料" || updated.Credential.State != CredentialValid {
		t.Fatalf("new credential state was not preserved: %#v", updated)
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
