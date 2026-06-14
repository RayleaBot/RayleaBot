package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

func newRevisionID(templateID, digest string) string {
	templateID = strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(strings.TrimSpace(templateID))
	if len(digest) > 8 {
		digest = digest[:8]
	}
	sequence := atomic.AddUint64(&revisionCounter, 1)
	return fmt.Sprintf("rev_%s_%s_%s_%06d", templateID, time.Now().UTC().Format("20060102T150405000000000"), digest, sequence)
}

func newStoredRevision(templateID, revisionID string, compiled *rendertemplates.CompiledTemplate, kind string, message *string, savedAt string) renderrepo.StoredTemplateRevision {
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

func newValidationStatus(valid bool, issueCount int) renderrepo.TemplateValidationStatus {
	return renderrepo.TemplateValidationStatus{
		Valid:      valid,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		IssueCount: issueCount,
	}
}

func issuesOrEmpty(issues []rendertemplates.TemplateValidationIssue) []rendertemplates.TemplateValidationIssue {
	if len(issues) == 0 {
		return []rendertemplates.TemplateValidationIssue{}
	}
	return issues
}
