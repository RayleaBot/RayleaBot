package render

import (
	"database/sql"
	"encoding/json"
)

func newStoredRevision(templateID, revisionID string, compiled *compiledTemplate, kind string, message *string, savedAt string) storedTemplateRevision {
	manifestJSON, _ := json.Marshal(compiled.bundle.normalizedManifest)
	inputSchemaJSON := sql.NullString{}
	if compiled.bundle.source.InputSchemaJSON != nil {
		encoded, _ := json.Marshal(compiled.bundle.source.InputSchemaJSON)
		inputSchemaJSON = sql.NullString{String: string(encoded), Valid: true}
	}

	return storedTemplateRevision{
		RevisionID:      revisionID,
		TemplateID:      templateID,
		TemplateVersion: compiled.bundle.manifest.Version,
		Kind:            kind,
		Message:         message,
		SavedAt:         savedAt,
		SourceDigest:    compiled.bundle.digest,
		ManifestJSON:    string(manifestJSON),
		HTML:            compiled.bundle.source.HTML,
		Stylesheet:      compiled.bundle.source.Stylesheet,
		InputSchemaJSON: inputSchemaJSON,
	}
}
