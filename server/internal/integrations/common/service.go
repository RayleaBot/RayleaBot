package common

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type Service struct {
	providers map[string]Provider
	now       func() time.Time
	mu        sync.Mutex
	sessions  map[string]LoginSession
}

type Options struct {
	Transport   http.RoundTripper
	Now         func() time.Time
	BrowserPath string
	BrowserArgs []string
}

func NewService(providers map[string]Provider, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		providers: providers,
		now:       now,
		sessions:  make(map[string]LoginSession),
	}
}

func (s *Service) Create(ctx context.Context, platform string) (CreateResult, error) {
	if s == nil {
		return CreateResult{}, ErrUnsupportedPlatform
	}
	platform, provider, err := s.provider(platform)
	if err != nil {
		return CreateResult{}, err
	}
	now := s.now().UTC()
	session, err := provider.Create(ctx, now)
	if err != nil {
		return CreateResult{}, err
	}
	session.Platform = platform
	session.State = NormalizeState(session.State)
	if session.State == "" {
		session.State = StatePendingScan
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = now.Add(3 * time.Minute)
	}
	loginID, err := RandomLoginID(platform)
	if err != nil {
		return CreateResult{}, err
	}
	session.LoginID = loginID
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	s.sessions[loginID] = session
	s.mu.Unlock()
	return CreateResultFromSession(session), nil
}

func (s *Service) Poll(ctx context.Context, platform, loginID string) (PollResult, error) {
	if s == nil {
		return PollResult{}, ErrUnsupportedPlatform
	}
	platform, provider, err := s.provider(platform)
	if err != nil {
		return PollResult{}, err
	}
	loginID = strings.TrimSpace(loginID)
	now := s.now().UTC()
	s.mu.Lock()
	session, ok := s.sessions[loginID]
	if !ok || session.Platform != platform {
		s.mu.Unlock()
		return PollResult{}, ErrLoginSessionNotFound
	}
	if now.After(session.ExpiresAt) && session.State != StateSucceeded {
		session.State = StateExpired
		s.sessions[loginID] = session
		result := PollResultFromSession(session)
		s.mu.Unlock()
		closeProviderSession(provider, session)
		return result, nil
	}
	if session.State == StateSucceeded || session.State == StateExpired {
		result := PollResultFromSession(session)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	next, err := provider.Poll(ctx, CloneSession(session), now)
	if err != nil {
		return PollResult{}, err
	}
	next.Platform = platform
	next.LoginID = loginID
	next.ExpiresAt = session.ExpiresAt
	next.QRCodeURL = session.QRCodeURL
	next.State = NormalizeState(next.State)
	if next.State == "" {
		next.State = session.State
	}
	s.mu.Lock()
	s.sessions[loginID] = next
	result := PollResultFromSession(next)
	s.mu.Unlock()
	if next.State == StateSucceeded || next.State == StateExpired {
		closeProviderSession(provider, next)
	}
	return result, nil
}

func (s *Service) provider(value string) (string, Provider, error) {
	platform, err := thirdparty.NormalizePlatform(value)
	if err != nil {
		return "", nil, err
	}
	provider := s.providers[platform]
	if provider == nil {
		return "", nil, ErrUnsupportedPlatform
	}
	return platform, provider, nil
}

func (s *Service) pruneExpiredLocked(now time.Time) {
	for loginID, session := range s.sessions {
		if now.After(session.ExpiresAt.Add(5 * time.Minute)) {
			delete(s.sessions, loginID)
			if provider := s.providers[session.Platform]; provider != nil {
				closeProviderSession(provider, session)
			}
		}
	}
}

func closeProviderSession(provider Provider, session LoginSession) {
	if closer, ok := provider.(ProviderSessionCloser); ok {
		closer.Close(session)
	}
}

func CloseProviderSession(provider Provider, session LoginSession) {
	closeProviderSession(provider, session)
}

func RecordCooldownError(mgr *CooldownManager, platform, accountID string, err error) {
	if mgr == nil || !IsRequestCooldownError(err) {
		return
	}
	key := fmt.Sprintf("%s:%s", strings.TrimSpace(platform), strings.TrimSpace(accountID))
	mgr.RecordError(key, err)
}
