package service

import "time"

func newValidationStatus(valid bool, issueCount int) TemplateValidationStatus {
	return TemplateValidationStatus{
		Valid:      valid,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		IssueCount: issueCount,
	}
}

func issuesOrEmpty(issues []TemplateValidationIssue) []TemplateValidationIssue {
	if len(issues) == 0 {
		return []TemplateValidationIssue{}
	}
	return issues
}
