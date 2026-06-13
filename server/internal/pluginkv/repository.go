package pluginkv

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

var (
	ErrValueTooLarge = errors.New("plugin kv value exceeds configured limit")
	ErrQuotaExceeded = errors.New("plugin kv total capacity exceeds configured limit")
)

type Limits struct {
	ValueMaxBytes int
	TotalMaxBytes int
}

type Repository interface {
	Get(context.Context, string, string) (any, bool, error)
	Set(context.Context, string, string, any, Limits) error
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
}

type SQLiteRepository struct {
	readQ  *sqlcgen.Queries
	writeQ *sqlcgen.Queries
	write  *sql.DB
	read   *sql.DB
	now    func() time.Time
}

func NewSQLiteRepository(store *storage.Store) (*SQLiteRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &SQLiteRepository{
		readQ:  sqlcgen.New(store.Read),
		writeQ: sqlcgen.New(store.Write),
		write:  store.Write,
		read:   store.Read,
		now:    time.Now,
	}, nil
}
