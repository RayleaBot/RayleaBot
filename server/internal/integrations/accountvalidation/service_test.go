package accountvalidation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type validationStoreStub struct {
	mu       sync.Mutex
	accounts []thirdparty.Account
	cookies  map[string]string
	updated  chan string
}

func (s *validationStoreStub) List(context.Context) ([]thirdparty.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]thirdparty.Account(nil), s.accounts...), nil
}

func (s *validationStoreStub) Get(_ context.Context, platform, accountID string) (thirdparty.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, account := range s.accounts {
		if account.Platform == platform && account.AccountID == accountID {
			return account, nil
		}
	}
	return thirdparty.Account{}, thirdparty.ErrAccountNotFound
}

func (s *validationStoreStub) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cookie := s.cookies[account.Platform+":"+account.AccountID]
	if cookie == "" {
		return "", errors.New("credential missing")
	}
	return cookie, nil
}

func (s *validationStoreStub) UpdateCredentialStatusIfUnchanged(_ context.Context, expected thirdparty.Account, profile thirdparty.AccountProfile, credential thirdparty.CredentialStatus) (thirdparty.Account, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, account := range s.accounts {
		if account.Platform != expected.Platform || account.AccountID != expected.AccountID {
			continue
		}
		if account.UpdatedAt != expected.UpdatedAt {
			return account, false, nil
		}
		account.Profile = thirdparty.MergeAccountProfiles(account.Profile, profile)
		account.Credential = credential
		s.accounts[index] = account
		if s.updated != nil {
			select {
			case s.updated <- account.Platform + ":" + account.AccountID:
			default:
			}
		}
		return account, true, nil
	}
	return thirdparty.Account{}, false, thirdparty.ErrAccountNotFound
}

type credentialValidatorFunc func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)

func (fn credentialValidatorFunc) CheckCookie(ctx context.Context, platform, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	return fn(ctx, platform, cookie)
}

func TestValidateAccountPersistsInvalidOutcomeAndKeepsProfile(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 8, 12, 8, 49, 13, 0, time.UTC)
	checkedAt := time.Date(2026, 8, 21, 1, 16, 16, 0, time.UTC)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Profile:    thirdparty.AccountProfile{UID: "123456", Nickname: "微博用户"},
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}, UpdatedAt: updatedAt,
		}},
		cookies: map[string]string{"weibo:primary": "SUB=expired;"},
	}
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		err := thirdparty.NewPlatformError("weibo", thirdparty.ErrorAuth, -100, 200, "微博账号 CK 已失效", nil)
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{
			State: thirdparty.CredentialInvalid, CheckedAt: &checkedAt, LastError: "微博账号 CK 已失效，请重新扫码",
		}, err
	})
	service := newService(store, validator, 360, nil, func() time.Time { return checkedAt })

	account, err := service.ValidateAccount(context.Background(), thirdparty.PlatformWeibo, "primary", TriggerManual)
	if err != nil {
		t.Fatalf("ValidateAccount returned error: %v", err)
	}
	if account.Credential.State != thirdparty.CredentialInvalid || account.Credential.LastError != "微博账号 CK 已失效，请重新扫码" {
		t.Fatalf("unexpected credential outcome: %#v", account.Credential)
	}
	if account.Profile.UID != "123456" || account.Profile.Nickname != "微博用户" {
		t.Fatalf("empty validation profile erased saved profile: %#v", account.Profile)
	}
}

func TestRunDueChecksOnlyEligibleExpiredAccounts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	old := now.Add(-7 * time.Hour)
	recent := now.Add(-time.Hour)
	updatedAt := now.Add(-24 * time.Hour)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{
			{Platform: thirdparty.PlatformWeibo, AccountID: "due-valid", Enabled: true, Configured: true, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &old}, UpdatedAt: updatedAt},
			{Platform: thirdparty.PlatformWeibo, AccountID: "due-unknown", Enabled: true, Configured: true, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialUnknown}, UpdatedAt: updatedAt},
			{Platform: thirdparty.PlatformWeibo, AccountID: "recent", Enabled: true, Configured: true, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &recent}, UpdatedAt: updatedAt},
			{Platform: thirdparty.PlatformWeibo, AccountID: "disabled", Enabled: false, Configured: true, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &old}, UpdatedAt: updatedAt},
			{Platform: thirdparty.PlatformWeibo, AccountID: "unconfigured", Enabled: true, Configured: false, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &old}, UpdatedAt: updatedAt},
			{Platform: thirdparty.PlatformWeibo, AccountID: "invalid", Enabled: true, Configured: true, Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialInvalid, CheckedAt: &old}, UpdatedAt: updatedAt},
		},
		cookies: map[string]string{
			"weibo:due-valid":   "SUB=valid;",
			"weibo:due-unknown": "SUB=unknown;",
		},
	}
	var checked []string
	var checkedMu sync.Mutex
	validator := credentialValidatorFunc(func(_ context.Context, platform, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		checkedMu.Lock()
		checked = append(checked, cookie)
		checkedMu.Unlock()
		checkedAt := now
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}, nil
	})
	service := newService(store, validator, 360, nil, func() time.Time { return now })

	service.runDue(context.Background(), TriggerScheduled, 6*time.Hour)

	checkedMu.Lock()
	defer checkedMu.Unlock()
	if len(checked) != 2 || checked[0] != "SUB=valid;" || checked[1] != "SUB=unknown;" {
		t.Fatalf("eligible checks = %#v", checked)
	}
}

func TestRunStartsWhenDisabledIntervalIsHotEnabled(t *testing.T) {
	t.Parallel()

	updatedAt := time.Now().Add(-time.Hour)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialUnknown}, UpdatedAt: updatedAt,
		}},
		cookies: map[string]string{"weibo:primary": "SUB=fixture;"},
		updated: make(chan string, 1),
	}
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		checkedAt := time.Now().UTC()
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}, nil
	})
	service := newService(store, validator, 0, nil, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	select {
	case <-store.updated:
		t.Fatal("disabled monitor performed a credential check")
	case <-time.After(20 * time.Millisecond):
	}
	service.SetIntervalMinutes(15)
	select {
	case key := <-store.updated:
		if key != "weibo:primary" {
			t.Fatalf("updated account = %q", key)
		}
	case <-time.After(time.Second):
		t.Fatal("hot-enabled monitor did not run a due check")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestRunDoesNotOverlapCredentialCheckCycles(t *testing.T) {
	t.Parallel()

	updatedAt := time.Now().Add(-time.Hour)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialUnknown}, UpdatedAt: updatedAt,
		}},
		cookies: map[string]string{"weibo:primary": "SUB=fixture;"},
	}
	var active atomic.Int32
	var maximum atomic.Int32
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(8 * time.Millisecond)
		active.Add(-1)
		checkedAt := time.Now().UTC()
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: thirdparty.CredentialUnknown, CheckedAt: &checkedAt}, nil
	})
	service := newService(store, validator, 1, nil, time.Now)
	service.intervalNanos.Store(int64(time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	if err := service.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent checks = %d, want 1", maximum.Load())
	}
}

func TestPluginValidationRequestRunsWhenScheduledChecksAreDisabled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 9, 20, 0, 0, time.UTC)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}, UpdatedAt: now.Add(-time.Hour),
		}},
		cookies: map[string]string{"weibo:primary": "SUB=fixture;"},
		updated: make(chan string, 1),
	}
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		checkedAt := now
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: thirdparty.CredentialInvalid, CheckedAt: &checkedAt}, nil
	})
	changed := make(chan struct{}, 1)
	service := newService(store, validator, 0, nil, func() time.Time { return now }, func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()

	accepted, reason, err := service.RequestPluginValidation(ctx, "raylea.subscription-hub", "weibo", "primary", "session_blocked", 432)
	if err != nil || !accepted || reason != "queued" {
		t.Fatalf("RequestPluginValidation = accepted %v, reason %q, err %v", accepted, reason, err)
	}
	select {
	case <-store.updated:
	case <-time.After(time.Second):
		t.Fatal("plugin validation request was not processed")
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("applied validation did not publish an account change")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestPluginValidationRequestDebouncesSameAccount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 9, 20, 0, 0, time.UTC)
	store := &validationStoreStub{accounts: []thirdparty.Account{{
		Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
		Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}, UpdatedAt: now,
	}}}
	service := newService(store, credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, nil
	}), 0, nil, func() time.Time { return now })

	accepted, reason, err := service.RequestPluginValidation(context.Background(), "plugin.one", "weibo", "primary", "auth_rejected", 401)
	if err != nil || !accepted || reason != "queued" {
		t.Fatalf("first request = accepted %v, reason %q, err %v", accepted, reason, err)
	}
	accepted, reason, err = service.RequestPluginValidation(context.Background(), "plugin.two", "weibo", "primary", "session_blocked", 432)
	if err != nil || accepted || reason != "debounced" {
		t.Fatalf("duplicate request = accepted %v, reason %q, err %v", accepted, reason, err)
	}
	store.mu.Lock()
	store.accounts[0].UpdatedAt = now.Add(time.Second)
	store.mu.Unlock()
	accepted, reason, err = service.RequestPluginValidation(context.Background(), "plugin.two", "weibo", "primary", "auth_rejected", 401)
	if err != nil || !accepted || reason != "queued" {
		t.Fatalf("new credential version request = accepted %v, reason %q, err %v", accepted, reason, err)
	}
}

func TestPluginValidationRequestRejectsUnavailableAccount(t *testing.T) {
	t.Parallel()

	service := newService(&validationStoreStub{}, credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, nil
	}), 0, nil, time.Now)
	accepted, reason, err := service.RequestPluginValidation(context.Background(), "plugin.one", "weibo", "missing", "auth_rejected", 401)
	if err != nil || accepted || reason != "account_unavailable" {
		t.Fatalf("unavailable request = accepted %v, reason %q, err %v", accepted, reason, err)
	}
}

func TestValidateAccountSerializesSameAccount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 9, 20, 0, 0, time.UTC)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}, UpdatedAt: now,
		}},
		cookies: map[string]string{"weibo:primary": "SUB=fixture;"},
	}
	var active atomic.Int32
	var maximum atomic.Int32
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
		active.Add(-1)
		checkedAt := now
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &checkedAt}, nil
	})
	service := newService(store, validator, 0, nil, func() time.Time { return now })
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = service.ValidateAccount(context.Background(), "weibo", "primary", TriggerManual)
		}()
	}
	close(start)
	wg.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent same-account checks = %d, want 1", maximum.Load())
	}
}
