package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const templateManifestFilename = "template.json"

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

type TemplateDraft struct {
	Source TemplateSource `json:"source"`
}

type TemplateSource struct {
	ManifestJSON    map[string]any `json:"manifest_json"`
	HTML            string         `json:"html"`
	Stylesheet      string         `json:"stylesheet"`
	InputSchemaJSON map[string]any `json:"input_schema_json"`
}

type TemplateFiles struct {
	Manifest    string  `json:"manifest"`
	HTML        string  `json:"html"`
	Stylesheet  string  `json:"stylesheet"`
	InputSchema *string `json:"input_schema"`
}

type TemplateValidationStatus struct {
	Valid      bool   `json:"valid"`
	CheckedAt  string `json:"checked_at"`
	IssueCount int    `json:"issue_count"`
}

type TemplateSourceInfo struct {
	Type     string `json:"type"`
	PluginID string `json:"plugin_id,omitempty"`
	LocalID  string `json:"local_id,omitempty"`
}

type TemplateVersion struct {
	RevisionID      string  `json:"revision_id"`
	TemplateVersion string  `json:"template_version"`
	SavedAt         string  `json:"saved_at"`
	Kind            string  `json:"kind"`
	Message         *string `json:"message"`
}

type TemplateSummary struct {
	ID                string `json:"id"`
	Version           string `json:"version"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	HasInputSchema    bool   `json:"has_input_schema"`
	CurrentRevisionID string `json:"current_revision_id"`
	UpdatedAt         string `json:"updated_at"`
	Source            TemplateSourceInfo
}

type TemplateDetail struct {
	TemplateSummary
	Files           TemplateFiles            `json:"files"`
	CurrentRevision TemplateVersion          `json:"current_revision"`
	LastValidation  TemplateValidationStatus `json:"last_validation"`
}

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type templateManifest struct {
	ID          string
	Version     string
	EntryHTML   string
	Stylesheet  string
	InputSchema *string
	Width       int
	Height      int
}

func (r *SQLiteTemplateRepository) SaveCurrentRevision(
	ctx context.Context,
	templateID string,
	baseRevisionID string,
	revision StoredTemplateRevision,
	validation TemplateValidationStatus,
) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin render template save transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	state, err := loadTemplateStateTx(ctx, tx, templateID)
	if err != nil {
		return err
	}
	if state.CurrentRevisionID != baseRevisionID {
		return &Error{
			Code:    "platform.template_revision_conflict",
			Message: "render template revision is stale",
		}
	}

	if err := insertTemplateRevision(ctx, tx, revision); err != nil {
		return err
	}
	if err := upsertTemplateState(ctx, tx, StoredTemplateState{
		TemplateID:           templateID,
		CurrentRevisionID:    revision.RevisionID,
		UpdatedAt:            revision.SavedAt,
		ValidationValid:      validation.Valid,
		ValidationCheckedAt:  validation.CheckedAt,
		ValidationIssueCount: validation.IssueCount,
		Source:               state.Source,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit render template save transaction: %w", err)
	}
	return nil
}

func (r *SQLiteTemplateRepository) UpdateValidationStatus(ctx context.Context, templateID string, validation TemplateValidationStatus) error {
	result, err := r.write.ExecContext(ctx, `
		UPDATE render_template_states
		SET validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?
		WHERE template_id = ?`,
		boolToInt(validation.Valid),
		validation.CheckedAt,
		validation.IssueCount,
		templateID,
	)
	if err != nil {
		return fmt.Errorf("update render template validation for %s: %w", templateID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read render template validation update rows for %s: %w", templateID, err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteTemplateRepository) RemovePluginTemplatesExcept(ctx context.Context, pluginID string, keepIDs []string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return nil
	}

	args := []any{pluginID}
	query := `DELETE FROM render_template_states WHERE source_type = 'plugin' AND source_plugin_id = ?`
	if len(keepIDs) > 0 {
		placeholders := make([]string, 0, len(keepIDs))
		seen := make(map[string]struct{}, len(keepIDs))
		for _, templateID := range keepIDs {
			templateID = strings.TrimSpace(templateID)
			if templateID == "" {
				continue
			}
			if _, ok := seen[templateID]; ok {
				continue
			}
			seen[templateID] = struct{}{}
			placeholders = append(placeholders, "?")
			args = append(args, templateID)
		}
		if len(placeholders) > 0 {
			query += ` AND template_id NOT IN (` + strings.Join(placeholders, ",") + `)`
		}
	}

	if _, err := r.write.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("remove stale plugin render templates for %s: %w", pluginID, err)
	}
	return nil
}

func (r *SQLiteTemplateRepository) RemovePluginTemplatesNotIn(ctx context.Context, activePluginIDs []string) error {
	seen := map[string]struct{}{}
	for _, pluginID := range activePluginIDs {
		pluginID = strings.TrimSpace(pluginID)
		if pluginID == "" {
			continue
		}
		seen[pluginID] = struct{}{}
	}
	if len(seen) == 0 {
		if _, err := r.write.ExecContext(ctx, `DELETE FROM render_template_states WHERE source_type = 'plugin'`); err != nil {
			return fmt.Errorf("remove all plugin render templates: %w", err)
		}
		return nil
	}

	args := make([]any, 0, len(seen))
	placeholders := make([]string, 0, len(seen))
	for pluginID := range seen {
		placeholders = append(placeholders, "?")
		args = append(args, pluginID)
	}
	if _, err := r.write.ExecContext(ctx,
		`DELETE FROM render_template_states WHERE source_type = 'plugin' AND source_plugin_id NOT IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	); err != nil {
		return fmt.Errorf("remove inactive plugin render templates: %w", err)
	}
	return nil
}
