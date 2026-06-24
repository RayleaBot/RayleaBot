package session

import (
	"context"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
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

func (p *Provider) Create(ctx context.Context, now time.Time) (common.LoginSession, error) {
	if p == nil || p.service == nil {
		return common.LoginSession{}, common.ErrUnsupportedPlatform
	}
	session, err := p.service.createRemoteSession(ctx, now)
	if err != nil {
		return common.LoginSession{}, err
	}
	return common.LoginSession{
		Platform:  thirdparty.PlatformBilibili,
		Token:     session.QRCodeKey,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     common.StatePendingScan,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session common.LoginSession, now time.Time) (common.LoginSession, error) {
	if p == nil || p.service == nil {
		return common.LoginSession{}, common.ErrUnsupportedPlatform
	}
	if now.After(session.ExpiresAt) && session.State != common.StateSucceeded {
		session.State = common.StateExpired
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
		return common.LoginSession{}, err
	}
	session.State = bilibiliToCommonState(next.State)
	session.Cookie = next.Cookie
	session.Account = next.Account
	return session, nil
}

func commonToBilibiliState(state string) string {
	switch state {
	case common.StatePendingConfirm:
		return QRLoginPendingConfirm
	case common.StateExpired:
		return QRLoginExpired
	case common.StateSucceeded:
		return QRLoginSucceeded
	default:
		return QRLoginPendingScan
	}
}

func bilibiliToCommonState(state string) string {
	switch state {
	case QRLoginPendingConfirm:
		return common.StatePendingConfirm
	case QRLoginExpired:
		return common.StateExpired
	case QRLoginSucceeded:
		return common.StateSucceeded
	default:
		return common.StatePendingScan
	}
}
