package managementhttp

import (
	"strings"
	"time"
)

func buildPluginSummary(catalog CatalogView, snapshot Snapshot) pluginSummaryResponse {
	if catalog == nil {
		return toPluginSummary(snapshot, nil)
	}
	conflicts := detectCommandConflicts(catalog.List())
	return toPluginSummary(snapshot, conflicts[snapshot.PluginID])
}

func toPluginSummary(snapshot Snapshot, conflicts []string) pluginSummaryResponse {
	role := effectivePluginRole(snapshot)
	return pluginSummaryResponse{
		ID:                snapshot.PluginID,
		Name:              pluginDisplayName(snapshot),
		Version:           strings.TrimSpace(snapshot.Version),
		Description:       strings.TrimSpace(snapshot.Description),
		Author:            strings.TrimSpace(snapshot.Author),
		Role:              role,
		RegistrationState: snapshot.RegistrationState,
		DesiredState:      snapshot.DesiredState,
		RuntimeState:      snapshot.RuntimeState,
		DisplayState:      snapshot.DisplayState,
		Source:            buildPluginSource(snapshot),
		Trust:             buildPluginTrust(role, snapshot),
		Commands:          buildPluginCommands(snapshot),
		Help:              buildPluginHelp(snapshot),
		CommandConflicts:  normalizeConflictList(conflicts),
		DeadLetter:        buildPluginDeadLetter(snapshot),
	}
}

func buildPluginDeadLetter(snapshot Snapshot) *pluginDeadLetterResponse {
	if snapshot.RuntimeState != "dead_letter" || snapshot.DeadLetter == nil {
		return nil
	}
	return &pluginDeadLetterResponse{
		EnteredAt:        snapshot.DeadLetter.EnteredAt.UTC().Format(time.RFC3339Nano),
		CrashCount:       snapshot.DeadLetter.CrashCount,
		LastErrorCode:    strings.TrimSpace(snapshot.DeadLetter.LastErrorCode),
		LastErrorMessage: strings.TrimSpace(snapshot.DeadLetter.LastErrorMessage),
	}
}

func normalizeConflictList(conflicts []string) []string {
	if len(conflicts) == 0 {
		return []string{}
	}
	return append([]string(nil), conflicts...)
}

func detectCommandConflicts(snapshots []Snapshot) map[string][]string {
	return DetectCommandConflicts(snapshots)
}
