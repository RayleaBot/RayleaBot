package service

import (
	"time"

	renderrepo "github.com/RayleaBot/RayleaBot/server/internal/render/repository"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

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
