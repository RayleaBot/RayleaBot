package thirdparty

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	QRLoginStatePendingScan          = "pending_scan"
	QRLoginStatePendingConfirm       = "pending_confirm"
	QRLoginStateVerificationRequired = "verification_required"
	QRLoginStateExpired              = "expired"
	QRLoginStateFailed               = "failed"
	QRLoginStateSucceeded            = "succeeded"
	qrLoginPersistTimeout            = 30 * time.Second
)

var (
	ErrQRLoginUnsupportedPlatform = errors.New("unsupported third-party qrcode login platform")
	ErrQRLoginSessionNotFound     = errors.New("third-party qrcode login session not found")
	ErrQRLoginCredentialMissing   = errors.New("third-party qrcode login credential missing")
	ErrQRLoginBrowserUnavailable  = errors.New("third-party qrcode login browser unavailable")
	ErrQRLoginBrowserBusy         = errors.New("third-party qrcode login browser profile busy")
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
	sessions  map[string]*qrLoginEntry
}

type qrLoginEntry struct {
	mu           sync.Mutex
	platform     string
	session      QRLoginSession
	closeSession QRLoginSession
	expiresAt    time.Time
	sessionCtx   context.Context
	cancel       context.CancelFunc
	cancelled    atomic.Bool
	closeOnce    sync.Once
}

type QRLoginServiceOption func(*QRLoginService)

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
		sessions:  make(map[string]*qrLoginEntry),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *QRLoginService) Create(ctx context.Context, platform string) (QRLoginCreateResult, error) {
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
		closeProviderSession(provider, session)
		return QRLoginCreateResult{}, err
	}
	session.LoginID = loginID
	sessionCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	stale := s.pruneExpiredLocked(now)
	s.sessions[loginID] = &qrLoginEntry{
		platform:     platform,
		session:      session,
		closeSession: CloneQRLoginSession(session),
		expiresAt:    session.ExpiresAt,
		sessionCtx:   sessionCtx,
		cancel:       cancel,
	}
	s.mu.Unlock()
	s.closeEntries(stale, true)
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
	platform, provider, err := s.provider(platform)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	loginID = strings.TrimSpace(loginID)
	now := s.now().UTC()
	s.mu.Lock()
	entry, ok := s.sessions[loginID]
	if !ok {
		s.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	s.mu.Unlock()

	entry.mu.Lock()
	if entry.cancelled.Load() {
		entry.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	session := entry.session
	if entry.platform != platform {
		entry.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	if IsQRLoginTerminalState(session.State) {
		result := QRLoginPollResultFromSession(session)
		entry.mu.Unlock()
		return result, nil
	}
	if now.After(session.ExpiresAt) && session.State != QRLoginStateSucceeded {
		session.State = QRLoginStateExpired
		entry.session = session
		result := QRLoginPollResultFromSession(session)
		entry.mu.Unlock()
		s.closeEntry(entry, provider, false)
		return result, nil
	}

	pollCtx, cancelPoll := context.WithCancel(ctx)
	stopSessionCancel := context.AfterFunc(entry.sessionCtx, cancelPoll)
	next, err := provider.Poll(pollCtx, CloneQRLoginSession(session), now)
	stopSessionCancel()
	cancelPoll()
	if entry.cancelled.Load() {
		entry.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	if err != nil {
		entry.mu.Unlock()
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
		if entry.cancelled.Load() || entry.sessionCtx.Err() != nil {
			entry.mu.Unlock()
			return QRLoginPollResult{}, ErrQRLoginSessionNotFound
		}
		persistCtx, cancelPersist := context.WithTimeout(entry.sessionCtx, qrLoginPersistTimeout)
		account, err := PersistQRLoginAccount(persistCtx, s.accounts, platform, next.Cookie, next.Account, now)
		cancelPersist()
		if entry.cancelled.Load() || entry.sessionCtx.Err() != nil {
			entry.mu.Unlock()
			return QRLoginPollResult{}, ErrQRLoginSessionNotFound
		}
		if err != nil {
			entry.mu.Unlock()
			return QRLoginPollResult{}, err
		}
		next.SavedAccount = &account
	}
	entry.session = next
	result := QRLoginPollResultFromSession(next)
	terminal := IsQRLoginTerminalState(next.State)
	entry.mu.Unlock()
	if terminal {
		s.closeEntry(entry, provider, false)
	}
	return result, nil
}

func (s *QRLoginService) Cancel(_ context.Context, platform, loginID string) error {
	platform, provider, err := s.provider(platform)
	if err != nil {
		return err
	}
	loginID = strings.TrimSpace(loginID)
	s.mu.Lock()
	entry, ok := s.sessions[loginID]
	if ok && entry.platform == platform {
		delete(s.sessions, loginID)
	}
	s.mu.Unlock()
	if !ok || entry.platform != platform {
		return ErrQRLoginSessionNotFound
	}
	s.closeEntry(entry, provider, true)
	entry.waitForPollCompletion()
	return nil
}

func (s *QRLoginService) Close() {
	s.mu.Lock()
	entries := make([]*qrLoginEntry, 0, len(s.sessions))
	for loginID, entry := range s.sessions {
		delete(s.sessions, loginID)
		entries = append(entries, entry)
	}
	s.mu.Unlock()
	s.closeEntries(entries, true)
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

func (s *QRLoginService) pruneExpiredLocked(now time.Time) []*qrLoginEntry {
	stale := make([]*qrLoginEntry, 0)
	for loginID, entry := range s.sessions {
		if now.After(entry.expiresAt.Add(5 * time.Minute)) {
			delete(s.sessions, loginID)
			stale = append(stale, entry)
		}
	}
	return stale
}

func (s *QRLoginService) closeEntries(entries []*qrLoginEntry, cancelled bool) {
	for _, entry := range entries {
		if provider := s.providers[entry.platform]; provider != nil {
			s.closeEntry(entry, provider, cancelled)
			if cancelled {
				entry.waitForPollCompletion()
			}
		}
	}
}

// waitForPollCompletion blocks until a Poll call holding the entry lock has returned.
func (entry *qrLoginEntry) waitForPollCompletion() {
	entry.mu.Lock()
	defer entry.mu.Unlock()
}

func (s *QRLoginService) closeEntry(entry *qrLoginEntry, provider QRLoginProvider, cancelled bool) {
	if entry == nil || provider == nil {
		return
	}
	if cancelled {
		entry.cancelled.Store(true)
	}
	entry.cancel()
	entry.closeOnce.Do(func() {
		closeProviderSession(provider, CloneQRLoginSession(entry.closeSession))
	})
}

func closeProviderSession(provider QRLoginProvider, session QRLoginSession) {
	if closer, ok := provider.(QRLoginProviderSessionCloser); ok {
		closer.Close(session)
	}
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
	case QRLoginStateVerificationRequired:
		return QRLoginStateVerificationRequired
	case QRLoginStateExpired:
		return QRLoginStateExpired
	case QRLoginStateFailed:
		return QRLoginStateFailed
	case QRLoginStateSucceeded:
		return QRLoginStateSucceeded
	default:
		return ""
	}
}

func IsQRLoginTerminalState(value string) bool {
	switch NormalizeQRLoginState(value) {
	case QRLoginStateExpired, QRLoginStateFailed, QRLoginStateSucceeded:
		return true
	default:
		return false
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
