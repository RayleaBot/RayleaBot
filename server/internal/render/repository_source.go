package render

import "context"

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
