package bilibili

import (
	"net/http"
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
		accountClient: NewAccountClient(transport, now, nil),
		now:           now,
		sessions:      make(map[string]qrLoginSession),
	}
}
