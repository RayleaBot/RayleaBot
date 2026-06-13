package render

import (
	"database/sql"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type sqliteTemplateRepository struct {
	read  *sql.DB
	write *sql.DB
}

type storedTemplateState struct {
	TemplateID           string
	CurrentRevisionID    string
	UpdatedAt            string
	ValidationValid      bool
	ValidationCheckedAt  string
	ValidationIssueCount int
	Source               TemplateSourceInfo
}

type storedTemplateRevision struct {
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

func newSQLiteTemplateRepository(store *storage.Store) (*sqliteTemplateRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}

	return &sqliteTemplateRepository{
		read:  store.Read,
		write: store.Write,
	}, nil
}
