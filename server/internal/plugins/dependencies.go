package plugins

import "strings"

// Requirement values of a manifest dependency: a missing required dependency blocks installation, a recommended
// one is only suggested.
const (
	DependencyRequired    = "required"
	DependencyRecommended = "recommended"
)

// Dependency is another plugin a manifest declares it relies on.
type Dependency struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
	Reason      string `json:"reason,omitempty"`
}

type DependencyMissingError struct {
	PluginIDs []string
}

func CheckRequiredDependencies(dependencies []Dependency, installed []Snapshot) *DependencyMissingError {
	available := make(map[string]bool, len(installed))
	for _, snapshot := range installed {
		if snapshot.Valid && snapshot.RegistrationState == RegistrationStateInstalled {
			available[snapshot.PluginID] = true
		}
	}
	var missing []string
	for _, dependency := range dependencies {
		if dependency.Requirement == DependencyRequired && !available[dependency.ID] {
			missing = append(missing, dependency.ID)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &DependencyMissingError{PluginIDs: missing}
}

func (e *DependencyMissingError) Error() string {
	return "请先安装前置插件：" + strings.Join(e.PluginIDs, "、")
}

func (e *DependencyMissingError) Details() map[string]any {
	return map[string]any{"plugin_ids": append([]string(nil), e.PluginIDs...)}
}
