package thirdpartylogin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type Service struct {
	providers map[string]provider
	now       func() time.Time
	mu        sync.Mutex
	sessions  map[string]loginSession
}

type Options struct {
	Transport   http.RoundTripper
	Now         func() time.Time
	BrowserPath string
	BrowserArgs []string

	douyinBrowser douyinLoginBrowser
}

func NewService(transport http.RoundTripper, now func() time.Time) *Service {
	return NewServiceWithOptions(Options{
		Transport: transport,
		Now:       now,
	})
}

func NewServiceWithOptions(options Options) *Service {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	client := newHTTPClient(options.Transport)
	douyinBrowser := options.douyinBrowser
	if douyinBrowser == nil {
		douyinBrowser = newChromedpDouyinBrowser(douyinBrowserOptions{
			BrowserPath: options.BrowserPath,
			BrowserArgs: options.BrowserArgs,
		})
	}
	return &Service{
		providers: map[string]provider{
			thirdparty.PlatformWeibo:        newWeiboProvider(client),
			thirdparty.PlatformDouyin:       newDouyinProvider(client, douyinBrowser),
			thirdparty.PlatformNeteaseMusic: newNeteaseMusicProvider(client),
		},
		now:      now,
		sessions: make(map[string]loginSession),
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
	session.State = normalizeState(session.State)
	if session.State == "" {
		session.State = StatePendingScan
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = now.Add(3 * time.Minute)
	}
	loginID, err := randomLoginID(platform)
	if err != nil {
		return CreateResult{}, err
	}
	session.LoginID = loginID
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	s.sessions[loginID] = session
	s.mu.Unlock()
	return createResult(session), nil
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
		result := pollResult(session)
		s.mu.Unlock()
		closeProviderSession(provider, session)
		return result, nil
	}
	if session.State == StateSucceeded || session.State == StateExpired {
		result := pollResult(session)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	next, err := provider.Poll(ctx, cloneSession(session), now)
	if err != nil {
		return PollResult{}, err
	}
	next.Platform = platform
	next.LoginID = loginID
	next.ExpiresAt = session.ExpiresAt
	next.QRCodeURL = session.QRCodeURL
	next.State = normalizeState(next.State)
	if next.State == "" {
		next.State = session.State
	}
	s.mu.Lock()
	s.sessions[loginID] = next
	result := pollResult(next)
	s.mu.Unlock()
	if next.State == StateSucceeded || next.State == StateExpired {
		closeProviderSession(provider, next)
	}
	return result, nil
}

func (s *Service) provider(value string) (string, provider, error) {
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

func closeProviderSession(provider provider, session loginSession) {
	if closer, ok := provider.(providerSessionCloser); ok {
		closer.Close(session)
	}
}

func randomLoginID(platform string) (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	prefix := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(platform)), "-", "_")
	if prefix == "" {
		prefix = "third_party"
	}
	return fmt.Sprintf("%s_qr_%s", prefix, hex.EncodeToString(bytes[:])), nil
}

func cloneSession(session loginSession) loginSession {
	session.Values = cloneStringMap(session.Values)
	session.Cookies = cloneStringMap(session.Cookies)
	return session
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func normalizeState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case StatePendingScan:
		return StatePendingScan
	case StatePendingConfirm:
		return StatePendingConfirm
	case StateExpired:
		return StateExpired
	case StateSucceeded:
		return StateSucceeded
	default:
		return ""
	}
}
