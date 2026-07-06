package thirdparty

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	QRLoginStatePendingScan    = "pending_scan"
	QRLoginStatePendingConfirm = "pending_confirm"
	QRLoginStateExpired        = "expired"
	QRLoginStateSucceeded      = "succeeded"
)

var (
	ErrQRLoginUnsupportedPlatform = errors.New("unsupported third-party qrcode login platform")
	ErrQRLoginSessionNotFound     = errors.New("third-party qrcode login session not found")
	ErrQRLoginCredentialMissing   = errors.New("third-party qrcode login credential missing")
)

type QRLoginCreateResult struct {
	Platform  string
	LoginID   string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
}

type QRLoginPollResult struct {
	Platform     string
	LoginID      string
	State        string
	ExpiresAt    time.Time
	Cookie       string
	Account      AccountProfile
	SavedAccount *Account
}

type QRLoginSession struct {
	Platform     string
	LoginID      string
	Token        string
	QRCodeURL    string
	ExpiresAt    time.Time
	State        string
	Cookie       string
	Account      AccountProfile
	SavedAccount *Account
	Values       map[string]string
	Cookies      map[string]string
}

type QRLoginProvider interface {
	Create(context.Context, time.Time) (QRLoginSession, error)
	Poll(context.Context, QRLoginSession, time.Time) (QRLoginSession, error)
}

type QRLoginProviderLoginIDPrefix interface {
	LoginIDPrefix() string
}

type QRLoginProviderSessionCloser interface {
	Close(QRLoginSession)
}

type QRLoginAccountStore interface {
	Upsert(context.Context, UpsertRequest) (Account, error)
}

type QRLoginService struct {
	providers map[string]QRLoginProvider
	accounts  QRLoginAccountStore
	now       func() time.Time
	mu        sync.Mutex
	sessions  map[string]QRLoginSession
}

type QRLoginServiceOption func(*QRLoginService)

type QRLoginOptions struct {
	Transport   http.RoundTripper
	Now         func() time.Time
	BrowserPath string
	BrowserArgs []string
}

func WithQRLoginAccountStore(accounts QRLoginAccountStore) QRLoginServiceOption {
	return func(service *QRLoginService) {
		service.accounts = accounts
	}
}

func NewQRLoginService(providers map[string]QRLoginProvider, now func() time.Time, options ...QRLoginServiceOption) *QRLoginService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	service := &QRLoginService{
		providers: providers,
		now:       now,
		sessions:  make(map[string]QRLoginSession),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *QRLoginService) Create(ctx context.Context, platform string) (QRLoginCreateResult, error) {
	if s == nil {
		return QRLoginCreateResult{}, ErrQRLoginUnsupportedPlatform
	}
	platform, provider, err := s.provider(platform)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	now := s.now().UTC()
	session, err := provider.Create(ctx, now)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session.Platform = platform
	session.State = NormalizeQRLoginState(session.State)
	if session.State == "" {
		session.State = QRLoginStatePendingScan
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = now.Add(3 * time.Minute)
	}
	loginID, err := providerLoginID(provider, platform)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session.LoginID = loginID
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	s.sessions[loginID] = session
	s.mu.Unlock()
	return QRLoginCreateResultFromSession(session), nil
}

func providerLoginID(provider QRLoginProvider, platform string) (string, error) {
	if prefixer, ok := provider.(QRLoginProviderLoginIDPrefix); ok {
		if prefix := strings.TrimSpace(prefixer.LoginIDPrefix()); prefix != "" {
			return RandomQRLoginIDWithPrefix(prefix)
		}
	}
	return RandomQRLoginID(platform)
}

func (s *QRLoginService) Poll(ctx context.Context, platform, loginID string) (QRLoginPollResult, error) {
	if s == nil {
		return QRLoginPollResult{}, ErrQRLoginUnsupportedPlatform
	}
	platform, provider, err := s.provider(platform)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	loginID = strings.TrimSpace(loginID)
	now := s.now().UTC()
	s.mu.Lock()
	session, ok := s.sessions[loginID]
	if !ok || session.Platform != platform {
		s.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	if now.After(session.ExpiresAt) && session.State != QRLoginStateSucceeded {
		session.State = QRLoginStateExpired
		s.sessions[loginID] = session
		result := QRLoginPollResultFromSession(session)
		s.mu.Unlock()
		closeProviderSession(provider, session)
		return result, nil
	}
	if session.State == QRLoginStateSucceeded || session.State == QRLoginStateExpired {
		result := QRLoginPollResultFromSession(session)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	next, err := provider.Poll(ctx, CloneQRLoginSession(session), now)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	next.Platform = platform
	next.LoginID = loginID
	next.ExpiresAt = session.ExpiresAt
	next.QRCodeURL = session.QRCodeURL
	next.State = NormalizeQRLoginState(next.State)
	if next.State == "" {
		next.State = session.State
	}
	if next.State == QRLoginStateSucceeded && s.accounts != nil {
		account, err := PersistQRLoginAccount(ctx, s.accounts, platform, next.Cookie, next.Account, now)
		if err != nil {
			return QRLoginPollResult{}, err
		}
		next.SavedAccount = &account
	}
	s.mu.Lock()
	s.sessions[loginID] = next
	result := QRLoginPollResultFromSession(next)
	s.mu.Unlock()
	if next.State == QRLoginStateSucceeded || next.State == QRLoginStateExpired {
		closeProviderSession(provider, next)
	}
	return result, nil
}

func (s *QRLoginService) provider(value string) (string, QRLoginProvider, error) {
	platform, err := NormalizePlatform(value)
	if err != nil {
		return "", nil, err
	}
	provider := s.providers[platform]
	if provider == nil {
		return "", nil, ErrQRLoginUnsupportedPlatform
	}
	return platform, provider, nil
}

func (s *QRLoginService) pruneExpiredLocked(now time.Time) {
	for loginID, session := range s.sessions {
		if now.After(session.ExpiresAt.Add(5 * time.Minute)) {
			delete(s.sessions, loginID)
			if provider := s.providers[session.Platform]; provider != nil {
				closeProviderSession(provider, session)
			}
		}
	}
}

func closeProviderSession(provider QRLoginProvider, session QRLoginSession) {
	if closer, ok := provider.(QRLoginProviderSessionCloser); ok {
		closer.Close(session)
	}
}

func CloseQRLoginProviderSession(provider QRLoginProvider, session QRLoginSession) {
	closeProviderSession(provider, session)
}

func QRLoginCreateResultFromSession(session QRLoginSession) QRLoginCreateResult {
	return QRLoginCreateResult{
		Platform:  session.Platform,
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}
}

func QRLoginPollResultFromSession(session QRLoginSession) QRLoginPollResult {
	return QRLoginPollResult{
		Platform:     session.Platform,
		LoginID:      session.LoginID,
		State:        session.State,
		ExpiresAt:    session.ExpiresAt,
		Cookie:       session.Cookie,
		Account:      session.Account,
		SavedAccount: session.SavedAccount,
	}
}

func NormalizeQRLoginState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case QRLoginStatePendingScan:
		return QRLoginStatePendingScan
	case QRLoginStatePendingConfirm:
		return QRLoginStatePendingConfirm
	case QRLoginStateExpired:
		return QRLoginStateExpired
	case QRLoginStateSucceeded:
		return QRLoginStateSucceeded
	default:
		return ""
	}
}

func RandomQRLoginID(platform string) (string, error) {
	prefix := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(platform)), "-", "_")
	if prefix == "" {
		prefix = "third_party"
	}
	return RandomQRLoginIDWithPrefix(prefix + "_qr")
}

func RandomQRLoginIDWithPrefix(prefix string) (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	prefix = strings.ReplaceAll(strings.TrimSpace(strings.ToLower(prefix)), "-", "_")
	if prefix == "" {
		prefix = "third_party_qr"
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes[:])), nil
}

func CloneQRLoginSession(session QRLoginSession) QRLoginSession {
	session.Values = CloneStringMap(session.Values)
	session.Cookies = CloneStringMap(session.Cookies)
	if session.SavedAccount != nil {
		account := *session.SavedAccount
		session.SavedAccount = &account
	}
	return session
}

func PersistQRLoginAccount(ctx context.Context, accounts QRLoginAccountStore, platform, cookie string, profile AccountProfile, now time.Time) (Account, error) {
	if accounts == nil {
		return Account{}, nil
	}
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return Account{}, ErrQRLoginCredentialMissing
	}
	accountID := qrLoginAccountID(profile)
	label := strings.TrimSpace(profile.Nickname)
	if label == "" {
		label = accountID
	}
	checkedAt := now.UTC()
	return accounts.Upsert(ctx, UpsertRequest{
		Platform:  platform,
		AccountID: accountID,
		Label:     label,
		Enabled:   true,
		Cookie:    cookie,
		Profile:   profile,
		Credential: CredentialStatus{
			State:     CredentialValid,
			CheckedAt: &checkedAt,
		},
	})
}

func qrLoginAccountID(profile AccountProfile) string {
	if accountID, err := NormalizeAccountID(profile.UID); err == nil {
		return accountID
	}
	return "primary"
}
