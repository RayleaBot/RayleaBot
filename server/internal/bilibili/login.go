package bilibili

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

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
	LoginID   string
	State     string
	ExpiresAt time.Time
	Cookie    string
	Account   thirdparty.AccountProfile
}

func NewQRLoginService(transport http.RoundTripper, now func() time.Time) *QRLoginService {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &QRLoginService{
		client:        &http.Client{Transport: transport, Timeout: defaultRequestTimeout},
		accountClient: NewAccountClient(transport, now),
		now:           now,
		sessions:      make(map[string]qrLoginSession),
	}
}

func (s *QRLoginService) Create(ctx context.Context) (QRLoginCreateResult, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, qrCodeGenerateURL, nil)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	applyBilibiliWebHeaders(request, http.MethodGet)
	response, err := s.client.Do(request)
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return QRLoginCreateResult{}, fmt.Errorf("bilibili qr generate http %d", response.StatusCode)
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
		return QRLoginCreateResult{}, err
	}
	if document.Code != 0 || strings.TrimSpace(document.Data.URL) == "" || strings.TrimSpace(document.Data.QRCodeKey) == "" {
		message := strings.TrimSpace(document.Message)
		if message == "" {
			message = "二维码创建失败"
		}
		return QRLoginCreateResult{}, fmt.Errorf("bilibili qr generate: %s", message)
	}
	loginID, err := randomLoginID()
	if err != nil {
		return QRLoginCreateResult{}, err
	}
	session := qrLoginSession{
		LoginID:   loginID,
		QRCodeKey: strings.TrimSpace(document.Data.QRCodeKey),
		QRCodeURL: strings.TrimSpace(document.Data.URL),
		ExpiresAt: s.now().UTC().Add(3 * time.Minute),
		State:     QRLoginPendingScan,
	}
	s.mu.Lock()
	s.pruneExpiredLocked()
	s.sessions[loginID] = session
	s.mu.Unlock()
	return QRLoginCreateResult{
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}, nil
}

func (s *QRLoginService) Poll(ctx context.Context, loginID string) (QRLoginPollResult, error) {
	loginID = strings.TrimSpace(loginID)
	s.mu.Lock()
	session, ok := s.sessions[loginID]
	if !ok {
		s.mu.Unlock()
		return QRLoginPollResult{}, fmt.Errorf("unknown qrcode login id")
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
	applyBilibiliWebHeaders(request, http.MethodGet)
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
		cookie, err := cookieFromLoginURL(document.Data.URL, document.Data.RefreshToken)
		if err != nil {
			return session, err
		}
		account, _, err := s.accountClient.CheckCookie(ctx, cookie)
		if err != nil {
			return session, err
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

func cookieFromLoginURL(rawURL, refreshToken string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	parts := []string{}
	for _, key := range []string{"SESSDATA", "bili_jct", "DedeUserID"} {
		value := strings.TrimSpace(query.Get(key))
		if value == "" {
			return "", fmt.Errorf("bilibili login missing %s", key)
		}
		parts = append(parts, key+"="+value)
	}
	if strings.TrimSpace(refreshToken) != "" {
		parts = append(parts, "ac_time_value="+strings.TrimSpace(refreshToken))
	}
	return strings.Join(parts, "; ") + ";", nil
}

func randomLoginID() (string, error) {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "qr_" + hex.EncodeToString(bytes[:]), nil
}

func decodeLimitedJSON(reader io.Reader, target any) error {
	body, err := io.ReadAll(io.LimitReader(reader, 2<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
