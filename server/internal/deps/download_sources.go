package deps

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	sourceProbeBytes          int64 = 1024 * 1024
	sourceProbePerSourceLimit       = 8 * time.Second
	sourceProbeOverallLimit         = 12 * time.Second
	sourceProbeCloseRatio           = 0.10
)

type sourceProbeResult struct {
	source      ResourceSource
	index       int
	bytesRead   int64
	duration    time.Duration
	bytesPerSec float64
	ok          bool
}

func selectDownloadSources(ctx context.Context, sources []ResourceSource) []ResourceSource {
	normalized := normalizedResourceSources(sources)
	if len(normalized) <= 1 {
		return normalized
	}
	ctx, cancel := context.WithTimeout(ctx, sourceProbeOverallLimit)
	defer cancel()

	results := make([]sourceProbeResult, len(normalized))
	var wg sync.WaitGroup
	for index, source := range normalized {
		results[index] = sourceProbeResult{source: source, index: index}
		wg.Add(1)
		go func(index int, source ResourceSource) {
			defer wg.Done()
			result := probeDownloadSource(ctx, source, index)
			results[index] = result
		}(index, source)
	}
	wg.Wait()

	successful := make([]sourceProbeResult, 0, len(results))
	failed := make([]sourceProbeResult, 0, len(results))
	for _, result := range results {
		if result.ok {
			successful = append(successful, result)
			continue
		}
		failed = append(failed, result)
	}
	if len(successful) == 0 {
		return normalized
	}
	sort.SliceStable(successful, func(i, j int) bool {
		if successful[i].bytesPerSec == successful[j].bytesPerSec {
			return successful[i].index < successful[j].index
		}
		return successful[i].bytesPerSec > successful[j].bytesPerSec
	})
	sort.SliceStable(failed, func(i, j int) bool {
		return failed[i].index < failed[j].index
	})
	ordered := restoreCloseProbeOrder(successful)
	ordered = append(ordered, failed...)
	selected := make([]ResourceSource, 0, len(ordered))
	for _, result := range ordered {
		selected = append(selected, result.source)
	}
	return selected
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
func probeDownloadSource(ctx context.Context, source ResourceSource, index int) sourceProbeResult {
	result := sourceProbeResult{source: source, index: index}
	probeCtx, cancel := context.WithTimeout(ctx, sourceProbePerSourceLimit)
	defer cancel()

	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, source.URL, nil)
	if err != nil {
		return result
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=0-%d", sourceProbeBytes-1))
	startedAt := time.Now()
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return result
	}
	limited := io.LimitReader(response.Body, sourceProbeBytes)
	bytesRead, err := io.Copy(io.Discard, limited)
	if err != nil || bytesRead <= 0 {
		return result
	}
	duration := time.Since(startedAt)
	if duration <= 0 {
		duration = time.Millisecond
	}
	result.bytesRead = bytesRead
	result.duration = duration
	result.bytesPerSec = float64(bytesRead) / duration.Seconds()
	result.ok = true
	return result
}
