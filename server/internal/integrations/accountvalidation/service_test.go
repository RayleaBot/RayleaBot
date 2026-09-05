package accountvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
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

func TestLogValidationKeepsAmbiguousHTTP432Unknown(t *testing.T) {
	var output bytes.Buffer
	service := newService(nil, nil, 0, slog.New(slog.NewJSONHandler(&output, nil)), time.Now)
	previous := thirdparty.Account{Platform: thirdparty.PlatformWeibo, AccountID: "primary", Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}}
	current := previous
	current.Credential = thirdparty.CredentialStatus{State: thirdparty.CredentialUnknown, LastError: "微博 CK 状态暂时无法确认，请稍后重试"}
	err := thirdparty.NewPlatformError(thirdparty.PlatformWeibo, thirdparty.ErrorRiskControl, 0, 432, "session blocked", nil)

	service.logValidation(TriggerScheduled, previous, current, true, err)
	message := output.String()
	for _, expected := range []string{`"new_state":"unknown"`, `"http_status":432`, `"level":"WARN"`, "暂时无法确认"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("log %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "已确认失效") {
		t.Fatalf("ambiguous HTTP 432 was logged as invalid: %s", message)
	}
}

func TestValidationLogsBackgroundFailuresAndRecovery(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	store := &validationStoreStub{
		accounts: []thirdparty.Account{{
			Platform: thirdparty.PlatformWeibo, AccountID: "primary", Enabled: true, Configured: true,
			Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid}, UpdatedAt: now,
		}},
		cookies: map[string]string{"weibo:primary": "SUB=fixture;"},
	}
	state := thirdparty.CredentialValid
	stale := false
	checks := 0
	validator := credentialValidatorFunc(func(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		checks++
		if stale {
			store.mu.Lock()
			store.accounts[0].UpdatedAt = now.Add(time.Hour)
			store.mu.Unlock()
		}
		return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{State: state, CheckedAt: &now}, nil
	})
	var output bytes.Buffer
	service := newService(store, validator, 0, slog.New(slog.NewJSONHandler(&output, nil)), func() time.Time { return now })
	steps := []struct {
		name, state, level string
		trigger            Trigger
		advance            time.Duration
		repeats, recovered int
		stale              bool
	}{
		{name: "unchanged background success", state: thirdparty.CredentialValid, trigger: TriggerScheduled},
		{name: "manual result", state: thirdparty.CredentialValid, trigger: TriggerManual, level: "INFO"},
		{name: "first failure", state: thirdparty.CredentialUnknown, trigger: TriggerPlugin, level: "WARN", repeats: 1},
		{name: "same failure", state: thirdparty.CredentialUnknown, trigger: TriggerPlugin, advance: time.Minute},
		{name: "periodic summary", state: thirdparty.CredentialUnknown, trigger: TriggerScheduled, advance: 5 * time.Minute, level: "WARN", repeats: 2},
		{name: "different failure", state: thirdparty.CredentialInvalid, trigger: TriggerManual, level: "WARN", repeats: 1},
		{name: "recovery", state: thirdparty.CredentialValid, trigger: TriggerScheduled, level: "INFO", recovered: 1},
		{name: "repeated success", state: thirdparty.CredentialValid, trigger: TriggerScheduled},
		{name: "stale failure", state: thirdparty.CredentialInvalid, trigger: TriggerPlugin, stale: true},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			output.Reset()
			now = now.Add(step.advance)
			state, stale = step.state, step.stale
			account, err := service.ValidateAccount(t.Context(), thirdparty.PlatformWeibo, "primary", step.trigger)
			if err != nil {
				t.Fatal(err)
			}
			wantState := step.state
			if stale {
				wantState = thirdparty.CredentialValid
			}
			if account.Credential.State != wantState {
				t.Fatalf("state = %s, want %s", account.Credential.State, wantState)
			}
			if step.level == "" {
				if output.Len() != 0 {
					t.Fatalf("unexpected default log: %s", output.String())
				}
				return
			}
			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("expected one result log: %v", err)
			}
			if record["level"] != step.level || record["new_state"] != wantState || record["account_id"] != "primary" {
				t.Fatalf("wrong log outcome: %#v", record)
			}
			if step.repeats > 0 && record["repeat_count"] != float64(step.repeats) {
				t.Fatalf("repeat count = %v, want %d", record["repeat_count"], step.repeats)
			}
			if step.recovered > 0 && record["recovered_count"] != float64(step.recovered) {
				t.Fatalf("recovered count = %v, want %d", record["recovered_count"], step.recovered)
			}
		})
	}
	if checks != len(steps) {
		t.Fatalf("logging suppression changed validation count: %d", checks)
	}
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
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	service := newService(store, validator, 0, logger, func() time.Time { return now }, func() {
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
	logOutput := logs.String()
	for _, expected := range []string{
		`"level":"WARN"`,
		`"trigger":"plugin"`,
		`"platform":"weibo"`,
		`"account_id":"primary"`,
		`"new_state":"invalid"`,
	} {
		if !strings.Contains(logOutput, expected) {
			t.Fatalf("plugin validation log missing %q: %s", expected, logOutput)
		}
	}
	if got := strings.Count(strings.TrimSpace(logOutput), "\n") + 1; got != 1 {
		t.Fatalf("want one validation result log, got %d: %s", got, logOutput)
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
