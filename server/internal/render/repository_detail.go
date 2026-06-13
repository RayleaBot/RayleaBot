package render

import "context"

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
			Source:            normalizedTemplateSourceInfo(state.Source),
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
