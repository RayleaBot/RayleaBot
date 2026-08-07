package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

var ErrQRLoginSessionNotFound = errors.New("bilibili qrcode login session not found")

const Platform = thirdparty.PlatformBilibili

const (
	qrCodeGenerateURL = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate?source=main-fe-header"
	qrCodePollURL     = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll"

	QRLoginPendingScan    = "pending_scan"
	QRLoginPendingConfirm = "pending_confirm"
	QRLoginExpired        = "expired"
	QRLoginSucceeded      = "succeeded"
)

type QRLoginService struct {
	client        *http.Client
	accountClient *AccountClient
	identity      *IdentityProvider
	now           func() time.Time
	mu            sync.Mutex
	sessions      map[string]qrLoginSession
}

type qrLoginSession struct {
	LoginID   string
	QRCodeKey string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
	Cookie    string
	Account   thirdparty.AccountProfile
}

type QRLoginCreateResult struct {
	LoginID   string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
}

type QRLoginPollResult struct {
	LoginID      string
	State        string
	ExpiresAt    time.Time
	Cookie       string
	Account      thirdparty.AccountProfile
	SavedAccount *thirdparty.Account
}

func NewQRLoginService(transport http.RoundTripper, now func() time.Time) *QRLoginService {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	identity := NewIdentityProvider(now)
	return &QRLoginService{
		client:        &http.Client{Transport: transport, Timeout: DefaultRequestTimeout},
		accountClient: NewAccountClient(transport, now, identity),
		identity:      identity,
		now:           now,
		sessions:      make(map[string]qrLoginSession),
	}
}

func createResult(session qrLoginSession) QRLoginCreateResult {
	return QRLoginCreateResult{
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}
}

func (s *QRLoginService) Create(ctx context.Context) (QRLoginCreateResult, error) {
	session, err := s.createRemoteSession(ctx, s.now().UTC())
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	loginID, err := randomLoginID()
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session.LoginID = loginID
	s.mu.Lock()
	s.pruneExpiredLocked()
	s.sessions[loginID] = session
	s.mu.Unlock()
	return createResult(session), nil
}

func (s *QRLoginService) createRemoteSession(ctx context.Context, now time.Time) (qrLoginSession, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodeGenerateURL, nil)
	if err != nil {
		return qrLoginSession{}, err
	}
	s.identity.ApplyHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return qrLoginSession{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return qrLoginSession{}, fmt.Errorf("bilibili qr generate http %d", response.StatusCode)
	}
	var document struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}
	if err := decodeLimitedJSON(response.Body, &document); err != nil {
		return qrLoginSession{}, err
	}
	if document.Code != 0 || strings.TrimSpace(document.Data.URL) == "" || strings.TrimSpace(document.Data.QRCodeKey) == "" {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码创建失败"
		}
		return qrLoginSession{}, fmt.Errorf("bilibili qr generate: %s", message)
	}
	return qrLoginSession{
		QRCodeKey: strings.TrimSpace(document.Data.QRCodeKey),
		QRCodeURL: strings.TrimSpace(document.Data.URL),
		ExpiresAt: now.UTC().Add(3 * time.Minute),
		State:     QRLoginPendingScan,
	}, nil
}

func (s *QRLoginService) Poll(ctx context.Context, loginID string) (QRLoginPollResult, error) {
	loginID = strings.TrimSpace(loginID)
	s.mu.Lock()
	session, ok := s.sessions[loginID]
	if !ok {
		s.mu.Unlock()
		return QRLoginPollResult{}, ErrQRLoginSessionNotFound
	}
	if s.now().After(session.ExpiresAt) && session.State != QRLoginSucceeded {
		session.State = QRLoginExpired
		s.sessions[loginID] = session
		result := pollResult(session)
		s.mu.Unlock()
		return result, nil
	}
	if session.State == QRLoginSucceeded || session.State == QRLoginExpired {
		result := pollResult(session)
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	next, err := s.pollRemote(ctx, session)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	s.mu.Lock()
	s.sessions[loginID] = next
	result := pollResult(next)
	s.mu.Unlock()
	return result, nil
}

func (s *QRLoginService) pollRemote(ctx context.Context, session qrLoginSession) (qrLoginSession, error) {
	values := url.Values{
		"qrcode_key": {session.QRCodeKey},
		"source":     {"main-fe-header"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodePollURL+"?"+values.Encode(), nil)
	if err != nil {
		return session, err
	}
	s.identity.ApplyHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return session, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return session, fmt.Errorf("bilibili qr poll http %d", response.StatusCode)
	}
	var document struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Code         int    `json:"code"`
			Message      string `json:"message"`
			URL          string `json:"url"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := decodeLimitedJSON(response.Body, &document); err != nil {
		return session, err
	}
	if document.Code != 0 {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码状态读取失败"
		}
		return session, fmt.Errorf("bilibili qr poll: %s", message)
	}
	switch document.Data.Code {
	case 86101:
		session.State = QRLoginPendingScan
	case 86090:
		session.State = QRLoginPendingConfirm
	case 86038:
		session.State = QRLoginExpired
	case 0:
		cookie, err := cookieFromLoginURL(document.Data.URL, document.Data.RefreshToken, response.Cookies())
		if err != nil {
			return session, err
		}
		account := thirdparty.AccountProfile{UID: cookieValues(cookie)["DedeUserID"]}
		if resolved, _, checkErr := s.accountClient.CheckCookie(ctx, cookie); checkErr == nil {
			account = resolved
		}
		session.State = QRLoginSucceeded
		session.Cookie = cookie
		session.Account = account
	default:
		message := strings.TrimSpace(document.Data.Message)
		if message == "" {
			message = "二维码状态读取失败"
		}
		return session, fmt.Errorf("bilibili qr poll code %d: %s", document.Data.Code, message)
	}
	return session, nil
}

func pollResult(session qrLoginSession) QRLoginPollResult {
	return QRLoginPollResult{
		LoginID:   session.LoginID,
		State:     session.State,
		ExpiresAt: session.ExpiresAt,
		Cookie:    session.Cookie,
		Account:   session.Account,
	}
}

func (s *QRLoginService) pruneExpiredLocked() {
	now := s.now()
	for loginID, session := range s.sessions {
		if now.After(session.ExpiresAt.Add(5 * time.Minute)) {
			delete(s.sessions, loginID)
		}
	}
}

func cookieFromLoginURL(rawURL, refreshToken string, responseCookies []*http.Cookie) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	values := map[string]string{}
	for _, key := range []string{"SESSDATA", "bili_jct", "DedeUserID", "DedeUserID__ckMd5", "sid"} {
		value := strings.TrimSpace(query.Get(key))
		if value != "" {
			values[key] = value
		}
	}
	for _, item := range responseCookies {
		if item == nil || item.MaxAge < 0 {
			continue
		}
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name != "" && value != "" {
			values[name] = value
		}
	}
	if strings.TrimSpace(refreshToken) != "" {
		values["ac_time_value"] = strings.TrimSpace(refreshToken)
	}
	for _, key := range []string{"SESSDATA", "bili_jct", "DedeUserID"} {
		if strings.TrimSpace(values[key]) == "" {
			return "", fmt.Errorf("bilibili login missing %s", key)
		}
	}
	return mergeCookieValues("", values), nil
}

func randomLoginID() (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "qr_" + hex.EncodeToString(bytes[:]), nil
}

type Provider struct {
	service *QRLoginService
}

func NewProvider(transport http.RoundTripper, now func() time.Time) *Provider {
	return &Provider{service: NewQRLoginService(transport, now)}
}

func (p *Provider) LoginIDPrefix() string {
	return "qr"
}

func (p *Provider) Create(ctx context.Context, now time.Time) (thirdparty.QRLoginSession, error) {
	if p.service == nil {
		return thirdparty.QRLoginSession{}, thirdparty.ErrQRLoginUnsupportedPlatform
	}
	session, err := p.service.createRemoteSession(ctx, now)
	if err != nil {
		return thirdparty.QRLoginSession{}, err
	}
	return thirdparty.QRLoginSession{
		Platform:  thirdparty.PlatformBilibili,
		Token:     session.QRCodeKey,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     thirdparty.QRLoginStatePendingScan,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session thirdparty.QRLoginSession, now time.Time) (thirdparty.QRLoginSession, error) {
	if p.service == nil {
		return thirdparty.QRLoginSession{}, thirdparty.ErrQRLoginUnsupportedPlatform
	}
	if now.After(session.ExpiresAt) && session.State != thirdparty.QRLoginStateSucceeded {
		session.State = thirdparty.QRLoginStateExpired
		return session, nil
	}
	next, err := p.service.pollRemote(ctx, qrLoginSession{
		LoginID:   session.LoginID,
		QRCodeKey: session.Token,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     commonToBilibiliState(session.State),
		Cookie:    session.Cookie,
		Account:   session.Account,
	})
	if err != nil {
		return thirdparty.QRLoginSession{}, err
	}
	session.State = bilibiliToCommonState(next.State)
	session.Cookie = next.Cookie
	session.Account = next.Account
	return session, nil
}

func commonToBilibiliState(state string) string {
	switch state {
	case thirdparty.QRLoginStatePendingConfirm:
		return QRLoginPendingConfirm
	case thirdparty.QRLoginStateExpired:
		return QRLoginExpired
	case thirdparty.QRLoginStateSucceeded:
		return QRLoginSucceeded
	default:
		return QRLoginPendingScan
	}
}

func bilibiliToCommonState(state string) string {
	switch state {
	case QRLoginPendingConfirm:
		return thirdparty.QRLoginStatePendingConfirm
	case QRLoginExpired:
		return thirdparty.QRLoginStateExpired
	case QRLoginSucceeded:
		return thirdparty.QRLoginStateSucceeded
	default:
		return thirdparty.QRLoginStatePendingScan
	}
}
