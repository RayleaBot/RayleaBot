package thirdpartylogin

import (
	"context"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	StatePendingScan    = "pending_scan"
	StatePendingConfirm = "pending_confirm"
	StateExpired        = "expired"
	StateSucceeded      = "succeeded"
)

var (
	ErrUnsupportedPlatform  = errors.New("unsupported third-party qrcode login platform")
	ErrLoginSessionNotFound = errors.New("third-party qrcode login session not found")
)

type CreateResult struct {
	Platform  string
	LoginID   string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
}

type PollResult struct {
	Platform  string
	LoginID   string
	State     string
	ExpiresAt time.Time
	Cookie    string
	Account   thirdparty.AccountProfile
}

type provider interface {
	Create(context.Context, time.Time) (loginSession, error)
	Poll(context.Context, loginSession, time.Time) (loginSession, error)
}

type providerSessionCloser interface {
	Close(loginSession)
}

type loginSession struct {
	Platform  string
	LoginID   string
	Token     string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
	Cookie    string
	Account   thirdparty.AccountProfile
	Values    map[string]string
	Cookies   map[string]string
}

func createResult(session loginSession) CreateResult {
	return CreateResult{
		Platform:  session.Platform,
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}
}

func pollResult(session loginSession) PollResult {
	return PollResult{
		Platform:  session.Platform,
		LoginID:   session.LoginID,
		State:     session.State,
		ExpiresAt: session.ExpiresAt,
		Cookie:    session.Cookie,
		Account:   session.Account,
	}
}
