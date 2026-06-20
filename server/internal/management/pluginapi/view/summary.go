package view

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func BuildSummary(catalog plugins.CatalogView, snapshot plugins.Snapshot) SummaryResponse {
	if catalog == nil {
		return ToSummary(snapshot, nil)
	}
	conflicts := plugins.DetectCommandConflicts(catalog.List())
	return ToSummary(snapshot, conflicts[snapshot.PluginID])
}

func ToSummary(snapshot plugins.Snapshot, conflicts []string) SummaryResponse {
	role := effectivePluginRole(snapshot)
	state, diagnosis := plugins.ProjectState(snapshot)
	return SummaryResponse{
		ID:               snapshot.PluginID,
		Name:             pluginDisplayName(snapshot),
		Version:          strings.TrimSpace(snapshot.Version),
		Description:      strings.TrimSpace(snapshot.Description),
		Author:           strings.TrimSpace(snapshot.Author),
		Role:             role,
		State:            state,
		StateDiagnosis:   diagnosis,
		Source:           buildPluginSource(snapshot),
		Trust:            buildPluginTrust(role, snapshot),
		Commands:         buildPluginCommands(snapshot),
		Help:             buildPluginHelp(snapshot),
		CommandConflicts: normalizeConflictList(conflicts),
	}
}

func normalizeConflictList(conflicts []string) []string {
	if len(conflicts) == 0 {
		return []string{}
	}
	return append([]string(nil), conflicts...)
}
