package render

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type templateRepository struct {
	readQ *sqlcgen.Queries
	write *sql.DB
}

type currentTemplate struct {
	ID           string
	SourceDigest string
	UpdatedAt    string
	Source       TemplateSource
	Owner        TemplateSourceInfo
}

func newTemplateRepository(store *storage.Store) (*templateRepository, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	return &templateRepository{readQ: sqlcgen.New(store.Read), write: store.Write}, nil
}

func (r *templateRepository) SyncTemplate(ctx context.Context, item currentTemplate) (bool, error) {
	manifest, err := json.Marshal(item.Source.ManifestJSON)
	if err != nil {
		return false, err
	}
	var schema sql.NullString
	if item.Source.InputSchemaJSON != nil {
		encoded, err := json.Marshal(item.Source.InputSchemaJSON)
		if err != nil {
			return false, err
		}
		schema = sql.NullString{String: string(encoded), Valid: true}
	}
	tx, err := r.write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func(release func() error) { _ = release() }(tx.Rollback)
	queries := sqlcgen.New(tx)
	current, err := queries.GetRenderTemplate(ctx, item.ID)
	if err == nil {
		if current.SourceType != item.Owner.Type || current.SourcePluginID.String != item.Owner.PluginID || current.SourceLocalID.String != item.Owner.LocalID {
			return false, fmt.Errorf("render template %s is already registered by another source", item.ID)
		}
		if current.SourceDigest == item.SourceDigest {
			return false, tx.Commit()
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = queries.UpsertRenderTemplate(ctx, sqlcgen.UpsertRenderTemplateParams{
		TemplateID: item.ID, SourceDigest: item.SourceDigest, UpdatedAt: item.UpdatedAt,
		SourceType: item.Owner.Type, SourcePluginID: nullable(item.Owner.PluginID), SourceLocalID: nullable(item.Owner.LocalID),
		ManifestJson: string(manifest), Html: item.Source.HTML, Stylesheet: item.Source.Stylesheet, InputSchemaJson: schema,
	})
	if err != nil {
		return false, fmt.Errorf("sync current render template %s: %w", item.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func nullable(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

type storedManifest struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Version     string  `json:"version"`
	EntryHTML   string  `json:"entry_html"`
	Stylesheet  string  `json:"stylesheet"`
	InputSchema *string `json:"input_schema"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
}

func decodeDetail(row sqlcgen.RenderTemplate) (TemplateDetail, error) {
	detail := TemplateDetail{TemplateSummary: TemplateSummary{
		ID: row.TemplateID, SourceDigest: row.SourceDigest, UpdatedAt: row.UpdatedAt,
		HasInputSchema: row.InputSchemaJson.Valid,
		Source:         TemplateSourceInfo{Type: row.SourceType, PluginID: row.SourcePluginID.String, LocalID: row.SourceLocalID.String},
	}}
	var manifest storedManifest
	if err := json.Unmarshal([]byte(row.ManifestJson), &manifest); err != nil {
		return detail, err
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return detail, fmt.Errorf("current template %s has no name", detail.ID)
	}
	detail.Name, detail.Description, detail.Version = manifest.Name, manifest.Description, manifest.Version
	detail.Width, detail.Height = manifest.Width, manifest.Height
	detail.Files = TemplateFiles{Manifest: ManifestFilename, HTML: manifest.EntryHTML, Stylesheet: manifest.Stylesheet, InputSchema: manifest.InputSchema}
	return detail, nil
}

func (r *templateRepository) ListTemplateSummaries(ctx context.Context) ([]TemplateSummary, error) {
	rows, err := r.readQ.ListRenderTemplates(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]TemplateSummary, 0, len(rows))
	for _, row := range rows {
		detail, err := decodeDetail(row)
		if err != nil {
			return nil, err
		}
		items = append(items, detail.TemplateSummary)
	}
	return items, nil
}

func (r *templateRepository) GetTemplateDetail(ctx context.Context, id string) (TemplateDetail, error) {
	row, err := r.readQ.GetRenderTemplate(ctx, id)
	if err != nil {
		return TemplateDetail{}, err
	}
	return decodeDetail(row)
}

func (r *templateRepository) GetCurrentSource(ctx context.Context, id string) (string, TemplateSource, error) {
	var source TemplateSource
	row, err := r.readQ.GetRenderTemplate(ctx, id)
	if err != nil {
		return "", source, err
	}
	source.HTML, source.Stylesheet = row.Html, row.Stylesheet
	if err := json.Unmarshal([]byte(row.ManifestJson), &source.ManifestJSON); err != nil {
		return "", source, err
	}
	if row.InputSchemaJson.Valid {
		if err := json.Unmarshal([]byte(row.InputSchemaJson.String), &source.InputSchemaJSON); err != nil {
			return "", source, err
		}
	}
	return row.SourceDigest, source, nil
}

func (r *templateRepository) removeExcept(ctx context.Context, condition string, args []any, column string, keep []string) error {
	query := `DELETE FROM render_templates WHERE ` + condition
	if len(keep) > 0 {
		query += ` AND ` + column + ` NOT IN (` + strings.TrimRight(strings.Repeat("?,", len(keep)), ",") + `)`
		for _, value := range keep {
			args = append(args, value)
		}
	}
	_, err := r.write.ExecContext(ctx, query, args...)
	return err
}

func (r *templateRepository) RemoveSystemTemplatesExcept(ctx context.Context, ids []string) error {
	return r.removeExcept(ctx, `source_type = 'system'`, nil, "template_id", ids)
}
func (r *templateRepository) RemovePluginTemplatesExcept(ctx context.Context, pluginID string, ids []string) error {
	return r.removeExcept(ctx, `source_type = 'plugin' AND source_plugin_id = ?`, []any{pluginID}, "template_id", ids)
}
func (r *templateRepository) RemovePluginTemplatesNotIn(ctx context.Context, ids []string) error {
	return r.removeExcept(ctx, `source_type = 'plugin'`, nil, "source_plugin_id", ids)
}
