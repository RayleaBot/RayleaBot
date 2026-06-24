package common

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	StatePendingScan    = "pending_scan"
	StatePendingConfirm = "pending_confirm"
	StateExpired        = "expired"
	StateSucceeded      = "succeeded"
)

var (
	ErrUnsupportedPlatform    = errors.New("unsupported third-party qrcode login platform")
	ErrLoginSessionNotFound   = errors.New("third-party qrcode login session not found")
	ErrLoginCredentialMissing = errors.New("third-party qrcode login credential missing")
)

type CreateResult struct {
	Platform  string
	LoginID   string
	QRCodeURL string
	ExpiresAt time.Time
	State     string
}

type PollResult struct {
	Platform     string
	LoginID      string
	State        string
	ExpiresAt    time.Time
	Cookie       string
	Account      thirdparty.AccountProfile
	SavedAccount *thirdparty.Account
}

type LoginSession struct {
	Platform     string
	LoginID      string
	Token        string
	QRCodeURL    string
	ExpiresAt    time.Time
	State        string
	Cookie       string
	Account      thirdparty.AccountProfile
	SavedAccount *thirdparty.Account
	Values       map[string]string
	Cookies      map[string]string
}

type Provider interface {
	Create(context.Context, time.Time) (LoginSession, error)
	Poll(context.Context, LoginSession, time.Time) (LoginSession, error)
}

type ProviderLoginIDPrefix interface {
	LoginIDPrefix() string
}

type ProviderSessionCloser interface {
	Close(LoginSession)
}

type AccountStore interface {
	Upsert(context.Context, thirdparty.UpsertRequest) (thirdparty.Account, error)
}

func CreateResultFromSession(session LoginSession) CreateResult {
	return CreateResult{
		Platform:  session.Platform,
		LoginID:   session.LoginID,
		QRCodeURL: session.QRCodeURL,
		ExpiresAt: session.ExpiresAt,
		State:     session.State,
	}
}

func PollResultFromSession(session LoginSession) PollResult {
	return PollResult{
		Platform:     session.Platform,
		LoginID:      session.LoginID,
		State:        session.State,
		ExpiresAt:    session.ExpiresAt,
		Cookie:       session.Cookie,
		Account:      session.Account,
		SavedAccount: session.SavedAccount,
	}
}

func NormalizeState(value string) string {
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

func RandomLoginID(platform string) (string, error) {
	prefix := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(platform)), "-", "_")
	if prefix == "" {
		prefix = "third_party"
	}
	return RandomLoginIDWithPrefix(prefix + "_qr")
}

func RandomLoginIDWithPrefix(prefix string) (string, error) {
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

func CloneSession(session LoginSession) LoginSession {
	session.Values = CloneStringMap(session.Values)
	session.Cookies = CloneStringMap(session.Cookies)
	if session.SavedAccount != nil {
		account := *session.SavedAccount
		session.SavedAccount = &account
	}
	return session
}
