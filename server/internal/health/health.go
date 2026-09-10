// Package health contains protocol-neutral diagnostic issues.
package health

type DiagnosticIssue struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Summary        string `json:"summary"`
	UserMessage    string `json:"user_message,omitempty"`
	Remediation    string `json:"remediation"`
	InternalReason string `json:"internal_reason,omitempty"`
}
