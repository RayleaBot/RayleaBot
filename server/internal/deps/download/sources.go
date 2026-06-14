package download

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"
)

const (
	sourceProbeBytes          int64 = 1024 * 1024
	sourceProbePerSourceLimit       = 8 * time.Second
	sourceProbeOverallLimit         = 12 * time.Second
	sourceProbeCloseRatio           = 0.10
)

type probeResult struct {
	source      manifest.ResourceSource
	index       int
	bytesRead   int64
	duration    time.Duration
	bytesPerSec float64
	ok          bool
}

func SelectSources(ctx context.Context, sources []manifest.ResourceSource) []manifest.ResourceSource {
	normalized := NormalizeSources(sources)
	if len(normalized) <= 1 {
		return normalized
	}
	ctx, cancel := context.WithTimeout(ctx, sourceProbeOverallLimit)
	defer cancel()

	results := make([]probeResult, len(normalized))
	var wg sync.WaitGroup
	for index, source := range normalized {
		results[index] = probeResult{source: source, index: index}
		wg.Add(1)
		go func(index int, source manifest.ResourceSource) {
			defer wg.Done()
			result := probeSource(ctx, source, index)
			results[index] = result
		}(index, source)
	}
	wg.Wait()

	successful := make([]probeResult, 0, len(results))
	failed := make([]probeResult, 0, len(results))
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
	ordered := restoreCloseOrder(successful)
	ordered = append(ordered, failed...)
	selected := make([]manifest.ResourceSource, 0, len(ordered))
	for _, result := range ordered {
		selected = append(selected, result.source)
	}
	return selected
}

func restoreCloseOrder(results []probeResult) []probeResult {
	if len(results) <= 1 {
		return append([]probeResult(nil), results...)
	}
	ordered := make([]probeResult, 0, len(results))
	group := make([]probeResult, 0, len(results))
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

func probeSource(ctx context.Context, source manifest.ResourceSource, index int) probeResult {
	result := probeResult{source: source, index: index}
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

func NormalizeSources(sources []manifest.ResourceSource) []manifest.ResourceSource {
	normalized := make([]manifest.ResourceSource, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.URL) == "" {
			continue
		}
		source.URL = strings.TrimSpace(source.URL)
		source.Label = strings.TrimSpace(source.Label)
		source.Kind = strings.TrimSpace(source.Kind)
		normalized = append(normalized, source)
	}
	return normalized
}

func SourceSummary(kind string, source manifest.ResourceSource) string {
	label := strings.TrimSpace(source.Label)
	if label == "" {
		return "正在下载 " + managedResourceLabel(kind)
	}
	return "正在从 " + label + " 下载 " + managedResourceLabel(kind)
}

func managedResourceLabel(kind string) string {
	switch kind {
	case "chromium":
		return "Chromium 浏览环境"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return "运行环境"
	}
}
