package thirdparty

import (
	"context"
	"strings"
	"sync"
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

func TestServiceSerializesConcurrentPollAndPersistsOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &blockingQRProvider{
		create: QRLoginSession{
			Platform:  PlatformDouyin,
			Token:     "fixture-token",
			QRCodeURL: "https://example.test/qr",
			ExpiresAt: now.Add(3 * time.Minute),
			State:     QRLoginStatePendingScan,
		},
		pollStarted: make(chan struct{}),
		releasePoll: make(chan struct{}),
	}
	accounts := &synchronizedAccountStore{}
	service := NewQRLoginService(map[string]QRLoginProvider{PlatformDouyin: provider}, func() time.Time { return now }, WithQRLoginAccountStore(accounts))
	created, err := service.Create(context.Background(), PlatformDouyin)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, pollErr := service.Poll(context.Background(), PlatformDouyin, created.LoginID)
			results <- pollErr
		}()
	}
	<-provider.pollStarted
	close(provider.releasePoll)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("Poll returned error: %v", err)
		}
	}

	provider.mu.Lock()
	pollCalls := provider.pollCalls
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if pollCalls != 1 {
		t.Fatalf("poll calls = %d, want 1", pollCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", closeCalls)
	}
	if accounts.Count() != 1 {
		t.Fatalf("saved requests = %d, want 1", accounts.Count())
	}
}

func TestServiceCancelAndCloseReleaseProviderOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &blockingQRProvider{create: QRLoginSession{
		Platform:  PlatformDouyin,
		Token:     "fixture-token",
		QRCodeURL: "https://example.test/qr",
		ExpiresAt: now.Add(3 * time.Minute),
		State:     QRLoginStatePendingScan,
	}}
	service := NewQRLoginService(map[string]QRLoginProvider{PlatformDouyin: provider}, func() time.Time { return now })
	created, err := service.Create(context.Background(), PlatformDouyin)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := service.Cancel(context.Background(), PlatformDouyin, created.LoginID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if err := service.Cancel(context.Background(), PlatformDouyin, created.LoginID); err != ErrQRLoginSessionNotFound {
		t.Fatalf("second Cancel error = %v, want %v", err, ErrQRLoginSessionNotFound)
	}
	service.Close()

	provider.mu.Lock()
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", closeCalls)
	}
}

func TestServiceCancelInterruptsConcurrentPollWithoutSaving(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &blockingQRProvider{
		create: QRLoginSession{
			Platform:  PlatformDouyin,
			Token:     "fixture-token",
			QRCodeURL: "https://example.test/qr",
			ExpiresAt: now.Add(3 * time.Minute),
			State:     QRLoginStatePendingScan,
		},
		pollStarted:  make(chan struct{}),
		releasePoll:  make(chan struct{}),
		closeStarted: make(chan struct{}),
	}
	accounts := &synchronizedAccountStore{}
	service := NewQRLoginService(map[string]QRLoginProvider{PlatformDouyin: provider}, func() time.Time { return now }, WithQRLoginAccountStore(accounts))
	created, err := service.Create(context.Background(), PlatformDouyin)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	pollResult := make(chan error, 1)
	go func() {
		_, pollErr := service.Poll(context.Background(), PlatformDouyin, created.LoginID)
		pollResult <- pollErr
	}()
	<-provider.pollStarted

	cancelResult := make(chan error, 1)
	go func() {
		cancelResult <- service.Cancel(context.Background(), PlatformDouyin, created.LoginID)
	}()
	select {
	case <-provider.closeStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Cancel to close the provider session")
	}
	select {
	case err := <-cancelResult:
		t.Fatalf("Cancel returned before the in-flight poll stopped: %v", err)
	default:
	}
	provider.mu.Lock()
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("close calls before poll release = %d, want 1", closeCalls)
	}
	close(provider.releasePoll)
	if err := <-pollResult; err != ErrQRLoginSessionNotFound {
		t.Fatalf("concurrent Poll error = %v, want %v", err, ErrQRLoginSessionNotFound)
	}
	if err := <-cancelResult; err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if accounts.Count() != 0 {
		t.Fatalf("saved requests = %d, want 0", accounts.Count())
	}
}

func TestServiceCancelWaitsForCredentialPersistenceToStop(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := stubProvider{
		create: QRLoginSession{
			Platform:  PlatformDouyin,
			Token:     "fixture-token",
			QRCodeURL: "https://example.test/qr",
			ExpiresAt: now.Add(3 * time.Minute),
			State:     QRLoginStatePendingScan,
		},
		poll: QRLoginSession{
			State:   QRLoginStateSucceeded,
			Cookie:  "sessionid=fixture;",
			Account: AccountProfile{UID: "123456", Nickname: "扫码账号"},
		},
	}
	accounts := &cancelAwareBlockingAccountStore{
		started:  make(chan struct{}),
		finished: make(chan struct{}),
	}
	service := NewQRLoginService(map[string]QRLoginProvider{PlatformDouyin: provider}, func() time.Time { return now }, WithQRLoginAccountStore(accounts))
	created, err := service.Create(context.Background(), PlatformDouyin)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	pollResult := make(chan error, 1)
	go func() {
		_, pollErr := service.Poll(context.Background(), PlatformDouyin, created.LoginID)
		pollResult <- pollErr
	}()
	<-accounts.started

	if err := service.Cancel(context.Background(), PlatformDouyin, created.LoginID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	select {
	case <-accounts.finished:
	default:
		t.Fatal("Cancel returned before credential persistence stopped")
	}
	if err := <-pollResult; err != ErrQRLoginSessionNotFound {
		t.Fatalf("Poll error = %v, want %v", err, ErrQRLoginSessionNotFound)
	}
	if accounts.Count() != 0 {
		t.Fatalf("persisted requests = %d, want 0", accounts.Count())
	}
}

func TestServiceExpiryAndCloseReleaseProviderOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &blockingQRProvider{create: QRLoginSession{
		Platform:  PlatformDouyin,
		Token:     "fixture-token",
		QRCodeURL: "https://example.test/qr",
		ExpiresAt: now.Add(time.Minute),
		State:     QRLoginStatePendingScan,
	}}
	service := NewQRLoginService(map[string]QRLoginProvider{PlatformDouyin: provider}, func() time.Time { return now })
	created, err := service.Create(context.Background(), PlatformDouyin)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	now = now.Add(2 * time.Minute)
	for range 2 {
		result, pollErr := service.Poll(context.Background(), PlatformDouyin, created.LoginID)
		if pollErr != nil {
			t.Fatalf("Poll returned error: %v", pollErr)
		}
		if result.State != QRLoginStateExpired {
			t.Fatalf("state = %q, want expired", result.State)
		}
	}
	service.Close()

	provider.mu.Lock()
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", closeCalls)
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

type blockingQRProvider struct {
	create       QRLoginSession
	pollStarted  chan struct{}
	releasePoll  chan struct{}
	closeStarted chan struct{}

	mu         sync.Mutex
	pollCalls  int
	closeCalls int
}

func (p *blockingQRProvider) Create(context.Context, time.Time) (QRLoginSession, error) {
	return p.create, nil
}

func (p *blockingQRProvider) Poll(context.Context, QRLoginSession, time.Time) (QRLoginSession, error) {
	p.mu.Lock()
	p.pollCalls++
	call := p.pollCalls
	p.mu.Unlock()
	if call == 1 && p.pollStarted != nil {
		close(p.pollStarted)
	}
	if p.releasePoll != nil {
		<-p.releasePoll
	}
	return QRLoginSession{
		State:   QRLoginStateSucceeded,
		Cookie:  "sessionid=fixture;",
		Account: AccountProfile{UID: "123456", Nickname: "扫码账号"},
	}, nil
}

func (p *blockingQRProvider) Close(QRLoginSession) {
	p.mu.Lock()
	p.closeCalls++
	p.mu.Unlock()
	if p.closeStarted != nil {
		close(p.closeStarted)
	}
}

func (p prefixedProvider) LoginIDPrefix() string {
	return p.prefix
}

type stubAccountStore struct {
	requests []UpsertRequest
}

type synchronizedAccountStore struct {
	mu       sync.Mutex
	requests []UpsertRequest
}

type cancelAwareBlockingAccountStore struct {
	started  chan struct{}
	finished chan struct{}
	mu       sync.Mutex
	requests []UpsertRequest
}

func (s *cancelAwareBlockingAccountStore) Upsert(ctx context.Context, request UpsertRequest) (Account, error) {
	close(s.started)
	defer close(s.finished)
	<-ctx.Done()
	if ctx.Err() == nil {
		s.mu.Lock()
		s.requests = append(s.requests, request)
		s.mu.Unlock()
	}
	return Account{}, ctx.Err()
}

func (s *cancelAwareBlockingAccountStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *synchronizedAccountStore) Upsert(_ context.Context, request UpsertRequest) (Account, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	return Account{
		Platform:   request.Platform,
		AccountID:  request.AccountID,
		Label:      request.Label,
		Enabled:    request.Enabled,
		Configured: request.Cookie != "",
		Profile:    request.Profile,
		Credential: request.Credential,
		UpdatedAt:  time.Date(2026, 8, 21, 8, 0, 1, 0, time.UTC),
	}, nil
}

func (s *synchronizedAccountStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
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
