package deps

import (
	"context"
	"math"
	"sort"

	depsdownload "github.com/RayleaBot/RayleaBot/server/internal/deps/download"
)

const sourceProbeCloseRatio = 0.10

type sourceProbeResult struct {
	source      ResourceSource
	index       int
	bytesPerSec float64
	ok          bool
}

func selectDownloadSources(ctx context.Context, sources []ResourceSource) []ResourceSource {
	return depsdownload.SelectSources(ctx, sources)
}

func restoreCloseProbeOrder(results []sourceProbeResult) []sourceProbeResult {
	if len(results) <= 1 {
		return append([]sourceProbeResult(nil), results...)
	}
	ordered := make([]sourceProbeResult, 0, len(results))
	group := make([]sourceProbeResult, 0, len(results))
	groupBest := 0.0
	flush := func() {
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].index < group[j].index
		})
		ordered = append(ordered, group...)
		group = nil
	}
	for _, result := range results {
		if len(group) == 0 {
			group = append(group, result)
			groupBest = result.bytesPerSec
			continue
		}
		if groupBest > 0 && math.Abs(groupBest-result.bytesPerSec)/groupBest <= sourceProbeCloseRatio {
			group = append(group, result)
			continue
		}
		flush()
		group = append(group, result)
		groupBest = result.bytesPerSec
	}
	if len(group) > 0 {
		flush()
	}
	return ordered
}
