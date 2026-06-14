package service

import (
	"database/sql"
	"encoding/json"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
)

func newStoredRevision(templateID, revisionID string, compiled *CompiledTemplate, kind string, message *string, savedAt string) renderrepo.StoredTemplateRevision {
	manifestJSON, _ := json.Marshal(compiled.Bundle.NormalizedManifest)
	inputSchemaJSON := sql.NullString{}
	if compiled.Bundle.Source.InputSchemaJSON != nil {
		encoded, _ := json.Marshal(compiled.Bundle.Source.InputSchemaJSON)
		inputSchemaJSON = sql.NullString{String: string(encoded), Valid: true}
	}

	return renderrepo.StoredTemplateRevision{
		RevisionID:      revisionID,
		TemplateID:      templateID,
		TemplateVersion: compiled.Bundle.Manifest.Version,
		Kind:            kind,
		Message:         message,
		SavedAt:         savedAt,
		SourceDigest:    compiled.Bundle.Digest,
		ManifestJSON:    string(manifestJSON),
		HTML:            compiled.Bundle.Source.HTML,
		Stylesheet:      compiled.Bundle.Source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}
}
