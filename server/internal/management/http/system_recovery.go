package managementhttp

import "github.com/RayleaBot/RayleaBot/server/internal/recovery"

const maxRecoveryConfirmNoteRunes = 500

type recoveryConfirmRequest struct {
	ReviewIDs []string `json:"review_ids"`
	Note      string   `json:"note,omitempty"`
}

func RecoverySummaryDetails(repoRoot string) map[string]any {
	return map[string]any{
		"resource_type": "recovery_summary",
		"path":          recovery.SummaryPath(repoRoot),
	}
}
