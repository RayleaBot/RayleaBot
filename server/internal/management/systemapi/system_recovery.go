package systemapi

const maxRecoveryConfirmNoteRunes = 500

type recoveryConfirmRequest struct {
	ReviewIDs []string `json:"review_ids"`
	Note      string   `json:"note,omitempty"`
}
