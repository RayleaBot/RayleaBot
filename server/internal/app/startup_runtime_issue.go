package app

import (
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
)

func startupRuntimeFailureIssue(kind string, err error) recovery.CompatibilityIssue {
	issue := recovery.CompatibilityIssue{
		Code:        "platform.resource_missing",
		Severity:    "warning",
		Summary:     deps.ManagedResourceLabel(kind) + "准备失败。",
		Remediation: deps.BootstrapRemediation(kind, "", ""),
	}

	var bootstrapErr *deps.BootstrapError
	if !errors.As(err, &bootstrapErr) {
		return issue
	}

	if summary := strings.TrimSpace(bootstrapErr.Message); summary != "" {
		issue.Summary = summary
		if !strings.HasSuffix(issue.Summary, "。") {
			issue.Summary += "。"
		}
	}
	if remediation := strings.TrimSpace(bootstrapErr.Remediation); remediation != "" {
		issue.Remediation = remediation
	}
	return issue
}
