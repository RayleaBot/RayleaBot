package render

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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

func (r *sqliteTemplateRepository) SeedTemplateIfMissing(ctx context.Context, revision storedTemplateRevision, validation TemplateValidationStatus) error {
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin render template seed transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT current_revision_id FROM render_template_states WHERE template_id = ?`, revision.TemplateID).Scan(&existing)
	switch {
	case err == nil:
		return tx.Commit()
	case errors.Is(err, sql.ErrNoRows):
	default:
		return fmt.Errorf("query render template state for %s: %w", revision.TemplateID, err)
	}

	if err := insertTemplateRevision(ctx, tx, revision); err != nil {
		return err
	}
	if err := insertTemplateState(ctx, tx, storedTemplateState{
		TemplateID:           revision.TemplateID,
		CurrentRevisionID:    revision.RevisionID,
		UpdatedAt:            revision.SavedAt,
		ValidationValid:      validation.Valid,
		ValidationCheckedAt:  validation.CheckedAt,
		ValidationIssueCount: validation.IssueCount,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit render template seed transaction: %w", err)
	}
	return nil
}

func (r *sqliteTemplateRepository) ListTemplateSummaries(ctx context.Context) ([]TemplateSummary, error) {
	rows, err := r.read.QueryContext(ctx, `
		SELECT
			s.template_id,
			s.current_revision_id,
			s.updated_at,
			r.template_version,
			r.manifest_json,
			r.input_schema_json
		FROM render_template_states s
		INNER JOIN render_template_revisions r ON r.revision_id = s.current_revision_id
		ORDER BY s.template_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query render template summaries: %w", err)
	}
	defer rows.Close()

	var items []TemplateSummary
	for rows.Next() {
		var (
			templateID        string
			currentRevisionID string
			updatedAt         string
			templateVersion   string
			manifestJSONText  string
			inputSchemaJSON   sql.NullString
		)
		if err := rows.Scan(&templateID, &currentRevisionID, &updatedAt, &templateVersion, &manifestJSONText, &inputSchemaJSON); err != nil {
			return nil, fmt.Errorf("scan render template summary: %w", err)
		}

		manifest, err := decodeStoredManifest(templateID, manifestJSONText)
		if err != nil {
			return nil, err
		}

		items = append(items, TemplateSummary{
			ID:                templateID,
			Version:           templateVersion,
			Width:             manifest.Width,
			Height:            manifest.Height,
			HasInputSchema:    inputSchemaJSON.Valid && inputSchemaJSON.String != "",
			CurrentRevisionID: currentRevisionID,
			UpdatedAt:         updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate render template summaries: %w", err)
	}

	return items, nil
}

func (r *sqliteTemplateRepository) GetTemplateDetail(ctx context.Context, templateID string) (TemplateDetail, error) {
	state, revision, err := r.loadCurrentTemplate(ctx, templateID)
	if err != nil {
		return TemplateDetail{}, err
	}

	manifest, err := decodeStoredManifest(templateID, revision.ManifestJSON)
	if err != nil {
		return TemplateDetail{}, err
	}

	return TemplateDetail{
		TemplateSummary: TemplateSummary{
			ID:                templateID,
			Version:           revision.TemplateVersion,
			Width:             manifest.Width,
			Height:            manifest.Height,
			HasInputSchema:    revision.InputSchemaJSON.Valid && revision.InputSchemaJSON.String != "",
			CurrentRevisionID: state.CurrentRevisionID,
			UpdatedAt:         state.UpdatedAt,
		},
		Files: TemplateFiles{
			Manifest:    templateManifestFilename,
			HTML:        manifest.EntryHTML,
			Stylesheet:  manifest.Stylesheet,
			InputSchema: manifest.InputSchema,
		},
		CurrentRevision: TemplateVersion{
			RevisionID:      revision.RevisionID,
			TemplateVersion: revision.TemplateVersion,
			SavedAt:         revision.SavedAt,
			Kind:            revision.Kind,
			Message:         revision.Message,
		},
		LastValidation: TemplateValidationStatus{
			Valid:      state.ValidationValid,
			CheckedAt:  state.ValidationCheckedAt,
			IssueCount: state.ValidationIssueCount,
		},
	}, nil
}

func (r *sqliteTemplateRepository) GetCurrentSource(ctx context.Context, templateID string) (string, TemplateSource, error) {
	_, revision, err := r.loadCurrentTemplate(ctx, templateID)
	if err != nil {
		return "", TemplateSource{}, err
	}

	source, err := decodeStoredSource(templateID, revision)
	if err != nil {
		return "", TemplateSource{}, err
	}
	return revision.RevisionID, source, nil
}

func (r *sqliteTemplateRepository) GetRevisionSource(ctx context.Context, templateID, revisionID string) (TemplateSource, error) {
	revision, err := r.loadRevision(ctx, templateID, revisionID)
	if err != nil {
		return TemplateSource{}, err
	}

	return decodeStoredSource(templateID, revision)
}

func (r *sqliteTemplateRepository) ListTemplateVersions(ctx context.Context, templateID string) ([]TemplateVersion, error) {
	if exists, err := r.templateExists(ctx, templateID); err != nil {
		return nil, err
	} else if !exists {
		return nil, sql.ErrNoRows
	}

	rows, err := r.read.QueryContext(ctx, `
		SELECT revision_id, template_version, saved_at, kind, message
		FROM render_template_revisions
		WHERE template_id = ?
		ORDER BY saved_at DESC, revision_id DESC`, templateID)
	if err != nil {
		return nil, fmt.Errorf("query render template versions for %s: %w", templateID, err)
	}
	defer rows.Close()

	var versions []TemplateVersion
	for rows.Next() {
		var (
			version TemplateVersion
			message sql.NullString
		)
		if err := rows.Scan(&version.RevisionID, &version.TemplateVersion, &version.SavedAt, &version.Kind, &message); err != nil {
			return nil, fmt.Errorf("scan render template version for %s: %w", templateID, err)
		}
		version.Message = nullStringPointer(message)
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate render template versions for %s: %w", templateID, err)
	}

	return versions, nil
}

func (r *sqliteTemplateRepository) SaveCurrentRevision(
	ctx context.Context,
	templateID string,
	baseRevisionID string,
	revision storedTemplateRevision,
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
	if err := upsertTemplateState(ctx, tx, storedTemplateState{
		TemplateID:           templateID,
		CurrentRevisionID:    revision.RevisionID,
		UpdatedAt:            revision.SavedAt,
		ValidationValid:      validation.Valid,
		ValidationCheckedAt:  validation.CheckedAt,
		ValidationIssueCount: validation.IssueCount,
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit render template save transaction: %w", err)
	}
	return nil
}

func (r *sqliteTemplateRepository) UpdateValidationStatus(ctx context.Context, templateID string, validation TemplateValidationStatus) error {
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

func (r *sqliteTemplateRepository) loadCurrentTemplate(ctx context.Context, templateID string) (storedTemplateState, storedTemplateRevision, error) {
	row := r.read.QueryRowContext(ctx, `
		SELECT
			s.template_id,
			s.current_revision_id,
			s.updated_at,
			s.validation_valid,
			s.validation_checked_at,
			s.validation_issue_count,
			r.revision_id,
			r.template_version,
			r.kind,
			r.message,
			r.saved_at,
			r.source_digest,
			r.manifest_json,
			r.html,
			r.stylesheet,
			r.input_schema_json
		FROM render_template_states s
		INNER JOIN render_template_revisions r ON r.revision_id = s.current_revision_id
		WHERE s.template_id = ?`, templateID)

	var (
		state       storedTemplateState
		revision    storedTemplateRevision
		message     sql.NullString
		inputSchema sql.NullString
	)
	if err := row.Scan(
		&state.TemplateID,
		&state.CurrentRevisionID,
		&state.UpdatedAt,
		&state.ValidationValid,
		&state.ValidationCheckedAt,
		&state.ValidationIssueCount,
		&revision.RevisionID,
		&revision.TemplateVersion,
		&revision.Kind,
		&message,
		&revision.SavedAt,
		&revision.SourceDigest,
		&revision.ManifestJSON,
		&revision.HTML,
		&revision.Stylesheet,
		&inputSchema,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateState{}, storedTemplateRevision{}, sql.ErrNoRows
		}
		return storedTemplateState{}, storedTemplateRevision{}, fmt.Errorf("query render template %s: %w", templateID, err)
	}

	revision.TemplateID = templateID
	revision.Message = nullStringPointer(message)
	revision.InputSchemaJSON = inputSchema
	return state, revision, nil
}

func (r *sqliteTemplateRepository) loadRevision(ctx context.Context, templateID, revisionID string) (storedTemplateRevision, error) {
	row := r.read.QueryRowContext(ctx, `
		SELECT revision_id, template_id, template_version, kind, message, saved_at, source_digest, manifest_json, html, stylesheet, input_schema_json
		FROM render_template_revisions
		WHERE template_id = ? AND revision_id = ?`, templateID, revisionID)

	var (
		revision    storedTemplateRevision
		message     sql.NullString
		inputSchema sql.NullString
	)
	if err := row.Scan(
		&revision.RevisionID,
		&revision.TemplateID,
		&revision.TemplateVersion,
		&revision.Kind,
		&message,
		&revision.SavedAt,
		&revision.SourceDigest,
		&revision.ManifestJSON,
		&revision.HTML,
		&revision.Stylesheet,
		&inputSchema,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateRevision{}, sql.ErrNoRows
		}
		return storedTemplateRevision{}, fmt.Errorf("query render template revision %s/%s: %w", templateID, revisionID, err)
	}

	revision.Message = nullStringPointer(message)
	revision.InputSchemaJSON = inputSchema
	return revision, nil
}

func (r *sqliteTemplateRepository) templateExists(ctx context.Context, templateID string) (bool, error) {
	var count int
	if err := r.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM render_template_states WHERE template_id = ?`, templateID).Scan(&count); err != nil {
		return false, fmt.Errorf("query render template existence for %s: %w", templateID, err)
	}
	return count > 0, nil
}

func insertTemplateRevision(ctx context.Context, tx *sql.Tx, revision storedTemplateRevision) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO render_template_revisions (
			revision_id,
			template_id,
			template_version,
			kind,
			message,
			saved_at,
			source_digest,
			manifest_json,
			html,
			stylesheet,
			input_schema_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.RevisionID,
		revision.TemplateID,
		revision.TemplateVersion,
		revision.Kind,
		pointerStringValue(revision.Message),
		revision.SavedAt,
		revision.SourceDigest,
		revision.ManifestJSON,
		revision.HTML,
		revision.Stylesheet,
		revision.InputSchemaJSON,
	)
	if err != nil {
		return fmt.Errorf("insert render template revision %s/%s: %w", revision.TemplateID, revision.RevisionID, err)
	}
	return nil
}

func insertTemplateState(ctx context.Context, tx *sql.Tx, state storedTemplateState) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO render_template_states (
			template_id,
			current_revision_id,
			updated_at,
			validation_valid,
			validation_checked_at,
			validation_issue_count
		) VALUES (?, ?, ?, ?, ?, ?)`,
		state.TemplateID,
		state.CurrentRevisionID,
		state.UpdatedAt,
		boolToInt(state.ValidationValid),
		state.ValidationCheckedAt,
		state.ValidationIssueCount,
	)
	if err != nil {
		return fmt.Errorf("insert render template state %s: %w", state.TemplateID, err)
	}
	return nil
}

func upsertTemplateState(ctx context.Context, tx *sql.Tx, state storedTemplateState) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE render_template_states
		SET current_revision_id = ?, updated_at = ?, validation_valid = ?, validation_checked_at = ?, validation_issue_count = ?
		WHERE template_id = ?`,
		state.CurrentRevisionID,
		state.UpdatedAt,
		boolToInt(state.ValidationValid),
		state.ValidationCheckedAt,
		state.ValidationIssueCount,
		state.TemplateID,
	)
	if err != nil {
		return fmt.Errorf("update render template state %s: %w", state.TemplateID, err)
	}
	return nil
}

func loadTemplateStateTx(ctx context.Context, tx *sql.Tx, templateID string) (storedTemplateState, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT template_id, current_revision_id, updated_at, validation_valid, validation_checked_at, validation_issue_count
		FROM render_template_states
		WHERE template_id = ?`, templateID)

	var state storedTemplateState
	if err := row.Scan(
		&state.TemplateID,
		&state.CurrentRevisionID,
		&state.UpdatedAt,
		&state.ValidationValid,
		&state.ValidationCheckedAt,
		&state.ValidationIssueCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedTemplateState{}, sql.ErrNoRows
		}
		return storedTemplateState{}, fmt.Errorf("query render template state %s: %w", templateID, err)
	}

	return state, nil
}

func decodeStoredManifest(templateID string, manifestJSONText string) (templateManifest, error) {
	var manifestJSON map[string]any
	if err := jsonUnmarshalObject([]byte(manifestJSONText), &manifestJSON); err != nil {
		return templateManifest{}, fmt.Errorf("decode stored render template manifest for %s: %w", templateID, err)
	}

	manifest, _, err := parseTemplateManifest(templateID, manifestJSON)
	if err != nil {
		return templateManifest{}, fmt.Errorf("decode stored render template manifest for %s: %w", templateID, err)
	}
	return manifest, nil
}

func decodeStoredSource(templateID string, revision storedTemplateRevision) (TemplateSource, error) {
	var manifestJSON map[string]any
	if err := jsonUnmarshalObject([]byte(revision.ManifestJSON), &manifestJSON); err != nil {
		return TemplateSource{}, fmt.Errorf("decode stored render template manifest for %s/%s: %w", templateID, revision.RevisionID, err)
	}

	var inputSchemaJSON map[string]any
	if revision.InputSchemaJSON.Valid && revision.InputSchemaJSON.String != "" {
		if err := jsonUnmarshalObject([]byte(revision.InputSchemaJSON.String), &inputSchemaJSON); err != nil {
			return TemplateSource{}, fmt.Errorf("decode stored render input schema for %s/%s: %w", templateID, revision.RevisionID, err)
		}
	}

	return TemplateSource{
		ManifestJSON:    manifestJSON,
		HTML:            revision.HTML,
		Stylesheet:      revision.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}, nil
}

func jsonUnmarshalObject(encoded []byte, target *map[string]any) error {
	if len(encoded) == 0 {
		*target = nil
		return nil
	}
	return json.Unmarshal(encoded, target)
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func pointerStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
