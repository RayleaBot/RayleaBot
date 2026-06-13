package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type BootstrapState struct {
	Identifier    string
	SecretDigest  []byte
	SigningKey    []byte
	InitializedAt time.Time
}

type Repository interface {
	LoadBootstrap(context.Context) (*BootstrapState, error)
	LoadSessions(context.Context) ([]Claims, error)
	SaveBootstrap(context.Context, BootstrapState, Claims) error
	UpdateBootstrapSecretDigest(context.Context, []byte) error
	SaveSession(context.Context, Claims) error
	DeleteSessions(context.Context, []string) error
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	write  *sql.DB
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		write:  store.Write,
	}, nil
}
