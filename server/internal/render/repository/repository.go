package repository

import (
	"database/sql"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type SQLiteTemplateRepository struct {
	read  *sql.DB
	write *sql.DB
}

type StoredTemplateState struct {
	TemplateID           string
	CurrentRevisionID    string
	UpdatedAt            string
	ValidationValid      bool
	ValidationCheckedAt  string
	ValidationIssueCount int
	Source               TemplateSourceInfo
}

type StoredTemplateRevision struct {
	RevisionID      string
	TemplateID      string
	TemplateVersion string
	Kind            string
	Message         *string
	SavedAt         string
	SourceDigest    string
	ManifestJSON    string
	HTML            string
	Stylesheet      string
	InputSchemaJSON sql.NullString
}

func NewSQLiteTemplateRepository(store *storage.Store) (*SQLiteTemplateRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &SQLiteTemplateRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}
