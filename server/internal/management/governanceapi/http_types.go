package governanceapi

type governanceEntryUpsertRequest struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
}

type governanceWhitelistStateUpdateRequest struct {
	Enabled *bool `json:"enabled"`
}
