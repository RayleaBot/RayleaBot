package model

import "github.com/RayleaBot/RayleaBot/server/internal/recovery"

type StatusSnapshot struct {
	Status          string
	AdapterState    string
	ActivePlugins   int
	UptimeSeconds   int64
	RecoverySummary *recovery.CompatibilitySummary
}

type ErrorReason string

const (
	ErrorReasonInternal        ErrorReason = "internal"
	ErrorReasonInvalidRequest  ErrorReason = "invalid_request"
	ErrorReasonResourceMissing ErrorReason = "resource_missing"
)

type Error struct {
	Reason  ErrorReason
	Details map[string]any
}

func InternalError() *Error {
	return &Error{Reason: ErrorReasonInternal}
}

func InvalidRequestError(details map[string]any) *Error {
	return &Error{Reason: ErrorReasonInvalidRequest, Details: details}
}

func ResourceMissingError(details map[string]any) *Error {
	return &Error{Reason: ErrorReasonResourceMissing, Details: details}
}

func RecoverySummaryDetails(repoRoot string) map[string]any {
	return map[string]any{
		"resource_type": "recovery_summary",
		"path":          recovery.SummaryPath(repoRoot),
	}
}
