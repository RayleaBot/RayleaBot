package session

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"time"
)

const Platform = thirdparty.PlatformBilibili

type Provider struct {
	service *QRLoginService
}

func NewProvider(transport http.RoundTripper, now func() time.Time) *Provider {
	return &Provider{service: NewQRLoginService(transport, now)}
}

func (p *Provider) LoginIDPrefix() string {
	return "qr"
}

func (p *Provider) Create(ctx context.Context, now time.Time) (qrcode.LoginSession, error) {
	if p == nil || p.service == nil {
		return qrcode.LoginSession{}, qrcode.ErrUnsupportedPlatform
	}
	session, err := p.service.createRemoteSession(ctx, now)
	if err != nil {
		return qrcode.LoginSession{}, err
	}
	return qrcode.LoginSession{
		Platform:  thirdparty.PlatformBilibili,
		Token:     session.QRCodeKey,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     qrcode.StatePendingScan,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session qrcode.LoginSession, now time.Time) (qrcode.LoginSession, error) {
	if p == nil || p.service == nil {
		return qrcode.LoginSession{}, qrcode.ErrUnsupportedPlatform
	}
	if now.After(session.ExpiresAt) && session.State != qrcode.StateSucceeded {
		session.State = qrcode.StateExpired
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
		return qrcode.LoginSession{}, err
	}
	session.State = bilibiliToCommonState(next.State)
	session.Cookie = next.Cookie
	session.Account = next.Account
	return session, nil
}

func commonToBilibiliState(state string) string {
	switch state {
	case qrcode.StatePendingConfirm:
		return QRLoginPendingConfirm
	case qrcode.StateExpired:
		return QRLoginExpired
	case qrcode.StateSucceeded:
		return QRLoginSucceeded
	default:
		return QRLoginPendingScan
	}
}

func bilibiliToCommonState(state string) string {
	switch state {
	case QRLoginPendingConfirm:
		return qrcode.StatePendingConfirm
	case QRLoginExpired:
		return qrcode.StateExpired
	case QRLoginSucceeded:
		return qrcode.StateSucceeded
	default:
		return qrcode.StatePendingScan
	}
}
