package plugins

import (
	"sort"
	"strings"
	"time"
)

func ComputeEffectiveGrants(snapshot Snapshot, configAutoCapabilities []string, persisted []PluginGrant) []EffectiveGrant {
	items := make(map[string]EffectiveGrant)
	scopeJSON := BuildScopeJSON(snapshot)

	for _, capability := range builtinAutoCapabilities(snapshot) {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   snapshot.PluginID,
			Capability: capability,
			Source:     GrantSourceBuiltinAuto,
			ScopeJSON:  scopeJSON,
		})
	}

	for _, capability := range dedupeCapabilities(configAutoCapabilities) {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   snapshot.PluginID,
			Capability: capability,
			Source:     GrantSourceConfigAuto,
			ScopeJSON:  scopeJSON,
		})
	}

	for _, grant := range persisted {
		putEffectiveGrant(items, EffectiveGrant{
			PluginID:   grant.PluginID,
			Capability: strings.TrimSpace(grant.Capability),
			GrantedAt:  cloneTimePointer(&grant.GrantedAt),
			ExpiresAt:  cloneTimePointer(grant.ExpiresAt),
			Source:     GrantSourcePersisted,
			ScopeJSON:  grant.ScopeJSON,
		})
	}

	effective := make([]EffectiveGrant, 0, len(items))
	for _, item := range items {
		effective = append(effective, item)
	}
	sort.Slice(effective, func(left, right int) bool {
		return effective[left].Capability < effective[right].Capability
	})
	return effective
}

func BuildPermissionSummaries(snapshot Snapshot, effectiveGrants []EffectiveGrant) []PermissionSummary {
	grants := make(map[string]EffectiveGrant, len(effectiveGrants))
	for _, grant := range effectiveGrants {
		grants[grant.Capability] = grant
	}

	summaries := make([]PermissionSummary, 0, len(snapshot.RequiredPermissions)+len(snapshot.OptionalPermissions))
	seen := make(map[string]struct{}, len(snapshot.RequiredPermissions)+len(snapshot.OptionalPermissions))
	for _, capability := range snapshot.RequiredPermissions {
		summaries = appendPermissionSummary(summaries, seen, grants, capability, PermissionRequirementRequired)
	}
	for _, capability := range snapshot.OptionalPermissions {
		summaries = appendPermissionSummary(summaries, seen, grants, capability, PermissionRequirementOptional)
	}
	return summaries
}

func appendPermissionSummary(
	summaries []PermissionSummary,
	seen map[string]struct{},
	grants map[string]EffectiveGrant,
	capability string,
	requirement PermissionRequirement,
) []PermissionSummary {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return summaries
	}
	if _, ok := seen[capability]; ok {
		return summaries
	}
	seen[capability] = struct{}{}

	summary := PermissionSummary{
		Capability:  capability,
		Requirement: requirement,
		Status:      PermissionStatusNotGranted,
		Source:      PermissionSourceNone,
	}
	if grant, ok := grants[capability]; ok {
		summary.Status = PermissionStatusGranted
		summary.Source = grantSourceAsPermissionSource(grant.Source)
		summary.ExpiresAt = cloneTimePointer(grant.ExpiresAt)
	}
	return append(summaries, summary)
}

func grantSourceAsPermissionSource(source GrantSource) PermissionSource {
	switch source {
	case GrantSourceBuiltinAuto:
		return PermissionSourceBuiltinAuto
	case GrantSourceConfigAuto:
		return PermissionSourceConfigAuto
	case GrantSourcePersisted:
		return PermissionSourcePersisted
	default:
		return PermissionSourceNone
	}
}

func putEffectiveGrant(items map[string]EffectiveGrant, grant EffectiveGrant) {
	capability := strings.TrimSpace(grant.Capability)
	if capability == "" {
		return
	}
	grant.Capability = capability
	current, exists := items[capability]
	if !exists || grantSourcePriority(grant.Source) < grantSourcePriority(current.Source) {
		items[capability] = grant
		return
	}
	if current.ScopeJSON == "" && strings.TrimSpace(grant.ScopeJSON) != "" {
		current.ScopeJSON = grant.ScopeJSON
		items[capability] = current
	}
}

func grantSourcePriority(source GrantSource) int {
	switch source {
	case GrantSourceBuiltinAuto:
		return 0
	case GrantSourceConfigAuto:
		return 1
	case GrantSourcePersisted:
		return 2
	default:
		return 99
	}
}

func builtinAutoCapabilities(snapshot Snapshot) []string {
	if summaryViewRole(snapshot) != "builtin" {
		return nil
	}
	return dedupeCapabilities(append(append([]string{}, snapshot.RequiredPermissions...), snapshot.OptionalPermissions...))
}

func dedupeCapabilities(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
