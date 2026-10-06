//go:build !linux

package diagnostics

import "context"

func renderLibraryIssues(context.Context) []Issue { return nil }
