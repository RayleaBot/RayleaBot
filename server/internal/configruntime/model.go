package configruntime

type ApplyEffects struct {
	AppliedNow            []string `json:"applied_now"`
	ReloadedNow           []string `json:"reloaded_now"`
	RestartRequiredFields []string `json:"restart_required_fields"`
}

func NewApplyEffects() ApplyEffects {
	return ApplyEffects{
		AppliedNow:            []string{},
		ReloadedNow:           []string{},
		RestartRequiredFields: []string{},
	}
}

func (e ApplyEffects) RestartRequired() bool {
	return len(e.RestartRequiredFields) > 0
}

type Document struct {
	Config         map[string]any
	RedactedFields []string
}

type UpdateResult struct {
	Document        Document
	RestartRequired bool
	ApplyEffects    ApplyEffects
}
