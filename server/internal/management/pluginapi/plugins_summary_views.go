package pluginapi

import (
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"strings"
	"time"
)

func buildPluginSummary(catalog plugins.CatalogView, snapshot plugins.Snapshot) pluginSummaryResponse {
	if catalog == nil {
		return toPluginSummary(snapshot, nil)
	}
	conflicts := plugins.DetectCommandConflicts(catalog.List())
	return toPluginSummary(snapshot, conflicts[snapshot.PluginID])
}

func toPluginSummary(snapshot plugins.Snapshot, conflicts []string) pluginSummaryResponse {
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

func buildPluginDeadLetter(snapshot plugins.Snapshot) *pluginDeadLetterResponse {
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
