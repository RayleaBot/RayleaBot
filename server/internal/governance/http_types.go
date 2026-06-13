package governance

type governanceEntryResponse = EntryResponse
type governanceBlacklistResponse = BlacklistSnapshot
type governanceWhitelistResponse = WhitelistSnapshot
type governanceWhitelistStateResponse = WhitelistStateResponse
type governanceCommandCooldownResponse = CommandCooldownResponse
type governanceCommandPolicyEntryResponse = CommandPolicyEntryResponse
type governanceCommandPolicyResponse = CommandPolicyResponse

type governanceEntryUpsertRequest struct {
	EntryType string `json:"entry_type"`
	TargetID  string `json:"target_id"`
	Reason    string `json:"reason"`
}

type governanceWhitelistStateUpdateRequest struct {
	Enabled *bool `json:"enabled"`
}
