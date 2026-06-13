package deps

import "strings"

func (e *BootstrapError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "managed runtime bootstrap failed"
}
func (e *BootstrapError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
func (e *BootstrapError) Details() map[string]any {
	if e == nil {
		return nil
	}
	details := map[string]any{
		"resource_kind": e.Kind,
		"stage":         e.Stage,
	}
	if strings.TrimSpace(e.SelectedSource) != "" {
		details["selected_source"] = e.SelectedSource
	}
	if len(e.AttemptedSources) > 0 {
		details["attempted_sources"] = append([]string(nil), e.AttemptedSources...)
	}
	if strings.TrimSpace(e.ArchivePath) != "" {
		details["archive_path"] = e.ArchivePath
	}
	if strings.TrimSpace(e.StoreRoot) != "" {
		details["store_root"] = e.StoreRoot
	}
	if strings.TrimSpace(e.Remediation) != "" {
		details["remediation"] = e.Remediation
	}
	return details
}
