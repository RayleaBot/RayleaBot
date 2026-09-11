package plugins

import (
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
)

// ListFilter narrows the plugin list. State accepts a projected state or
// "alert", which selects failed and invalid plugins and plugins with command
// conflicts; Source selects "official" or "community" plugins.
type ListFilter struct {
	State  string
	Source string
}

// ListPage applies the filter and free-text query, orders the plugins by id
// and returns the requested page. conflicts is the DetectCommandConflicts
// result for the same snapshots.
func ListPage(snapshots []Snapshot, conflicts map[string][]string, filter ListFilter, query pagination.Query) ([]Snapshot, pagination.Metadata) {
	filtered := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if !matchesListFilter(snapshot, conflicts[snapshot.PluginID], filter) {
			continue
		}
		if pagination.Matches(query.Text, snapshot.PluginID, snapshot.Name, snapshot.Description) {
			filtered = append(filtered, snapshot)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].PluginID < filtered[j].PluginID })
	return pagination.Slice(filtered, query)
}

func matchesListFilter(snapshot Snapshot, conflicts []string, filter ListFilter) bool {
	state, _ := ProjectState(snapshot)
	switch filter.State {
	case "":
	case "alert":
		if state != PluginStateFailed && state != PluginStateInvalid && len(conflicts) == 0 {
			return false
		}
	default:
		if state != filter.State {
			return false
		}
	}
	official := buildTrustView(summaryViewRole(snapshot), snapshot).Level == "official"
	switch filter.Source {
	case "official":
		return official
	case "community":
		return !official
	default:
		return true
	}
}
