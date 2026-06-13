package thirdparty

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const PlatformBilibili = "bilibili"

var accountIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)

var ErrInvalidAccount = errors.New("invalid third-party account")

const (
	CredentialUnknown = "unknown"
	CredentialValid   = "valid"
	CredentialInvalid = "invalid"
)

type Account struct {
	Platform   string
	AccountID  string
	Label      string
	Enabled    bool
	Configured bool
	SecretKey  string
	Profile    AccountProfile
	Credential CredentialStatus
	LastUsedAt *time.Time
	UpdatedAt  time.Time
}

type UpsertRequest struct {
	Platform   string
	AccountID  string
	Label      string
	Enabled    bool
	Cookie     string
	Profile    AccountProfile
	Credential CredentialStatus
	Validate   func(context.Context, string) (AccountProfile, CredentialStatus, error)
}

type AccountProfile struct {
	UID       string
	Nickname  string
	AvatarURL string
}

type CredentialStatus struct {
	State     string
	CheckedAt *time.Time
	LastError string
}

type Service struct {
	read    *sql.DB
	write   *sql.DB
	secrets secrets.Store
	now     func() time.Time
}

func NewService(store *storage.Store, secretStore secrets.Store) (*Service, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	if secretStore == nil {
		return nil, errors.New("secret store is required")
	}
	return &Service{
		read:    store.Read,
		write:   store.Write,
		secrets: secretStore,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}
