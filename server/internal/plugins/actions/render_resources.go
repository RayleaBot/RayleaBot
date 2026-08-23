package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

const (
	maxRenderImageResourceBytes      int64 = 16 << 20
	maxRenderImageResourceTotalBytes int64 = 96 << 20
	renderImageResourceTimeout             = 30 * time.Second
	renderImageResourceConcurrency         = 4
	maxRenderImageResourceRedirects        = 3
	renderImageResourceUserAgent           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
)

var errRenderImageResourceUnavailable = errors.New("render.image resource is unavailable")

type prefetchedRenderImageResource struct {
	resource       RenderImageResource
	candidateIndex int
	spec           pluginruntime.RenderImageResource
	requestIndex   int
}

type renderImageResourceFetchResult struct {
	prefetched *prefetchedRenderImageResource
	reason     string
	err        error
}

func prefetchRenderImageResources(ctx context.Context, deps Deps, req ActionRequest) ([]RenderImageResource, func(), error) {
	if len(req.Action.RenderResources) == 0 {
		return nil, func() {}, nil
	}
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "http.request") {
		return nil, func() {}, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: "render.image resources require the http.request capability",
		}
	}

	workspace, err := os.MkdirTemp("", "rayleabot-render-resources-*")
	if err != nil {
		return nil, func() {}, &pluginruntime.Error{Code: "plugin.internal_error", Message: "render.image resource workspace is unavailable", Err: err}
	}
	cleanup := func() {
		_ = os.RemoveAll(workspace)
	}

	resourceCtx, cancel := context.WithTimeout(ctx, renderImageResourceTimeout)
	defer cancel()
	cfg := currentConfig(deps)
	client := newHTTPClient(httpClientConfig{
		Timeout:              currentHTTPTimeout(cfg),
		MaxRetries:           0,
		MaxResponseBodyBytes: maxRenderImageResourceBytes,
		AllowPrivateHosts:    append([]string(nil), cfg.HTTP.AllowPrivateHosts...),
	})
	scopeHosts := deps.Capabilities.HTTPHosts(resourceCtx, req.PluginID)
	results := make([]renderImageResourceFetchResult, len(req.Action.RenderResources))
	semaphore := make(chan struct{}, renderImageResourceConcurrency)
	var wait sync.WaitGroup
	for index, spec := range req.Action.RenderResources {
		wait.Add(1)
		go func(index int, spec pluginruntime.RenderImageResource) {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-resourceCtx.Done():
				results[index].reason = "timeout"
				return
			}
			prefetched, reason, fetchErr := fetchRenderImageResource(resourceCtx, client, scopeHosts, workspace, index, spec, 0, maxRenderImageResourceBytes)
			results[index] = renderImageResourceFetchResult{prefetched: prefetched, reason: reason, err: fetchErr}
		}(index, spec)
	}
	wait.Wait()

	items := make([]prefetchedRenderImageResource, 0, len(results))
	for index, result := range results {
		if result.err != nil {
			cleanup()
			return nil, func() {}, renderImageResourceFetchError(result.err)
		}
		if result.prefetched == nil {
			logRenderImageResourceUnavailable(deps, req, req.Action.RenderResources[index].ID, result.reason)
			continue
		}
		items = append(items, *result.prefetched)
	}

	items, err = reduceRenderImageResourceSet(resourceCtx, client, scopeHosts, workspace, items)
	if err != nil {
		cleanup()
		return nil, func() {}, renderImageResourceFetchError(err)
	}
	resources := make([]RenderImageResource, 0, len(items))
	sort.Slice(items, func(i, j int) bool { return items[i].requestIndex < items[j].requestIndex })
	for _, item := range items {
		resources = append(resources, item.resource)
	}
	return resources, cleanup, nil
}

func fetchRenderImageResource(ctx context.Context, client *httpClient, scopeHosts []string, workspace string, requestIndex int, spec pluginruntime.RenderImageResource, startCandidate int, maxAcceptedBytes int64) (*prefetchedRenderImageResource, string, error) {
	candidates := append([]string{spec.URL}, spec.FallbackURLs...)
	lastReason := "unavailable"
	for candidateIndex := startCandidate; candidateIndex < len(candidates); candidateIndex++ {
		resource, reason, err := downloadRenderImageResourceCandidate(ctx, client, scopeHosts, workspace, requestIndex, candidateIndex, spec.ID, candidates[candidateIndex], spec.Referer)
		if err != nil {
			if errors.Is(err, errHTTPScopeViolation) || errors.Is(err, errHTTPInvalidRequest) || reason == "filesystem" {
				return nil, "", err
			}
			lastReason = reason
			continue
		}
		if resource.Size > maxAcceptedBytes {
			_ = os.Remove(resource.Path)
			lastReason = "request_limit"
			continue
		}
		return &prefetchedRenderImageResource{
			resource:       resource,
			candidateIndex: candidateIndex,
			spec:           spec,
			requestIndex:   requestIndex,
		}, "", nil
	}
	return nil, lastReason, nil
}

func downloadRenderImageResourceCandidate(ctx context.Context, client *httpClient, scopeHosts []string, workspace string, requestIndex, candidateIndex int, resourceID, sourceURL, referer string) (RenderImageResource, string, error) {
	currentURL := sourceURL
	for redirectCount := 0; redirectCount <= maxRenderImageResourceRedirects; redirectCount++ {
		downloadPath := filepath.Join(workspace, fmt.Sprintf("resource-%02d-%02d.download", requestIndex, candidateIndex))
		file, err := os.OpenFile(downloadPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return RenderImageResource{}, "filesystem", err
		}
		hash := sha256.New()
		response, requestErr := client.do(ctx, httpClientRequest{
			Method:             "GET",
			URL:                currentURL,
			Headers:            renderImageResourceHeaders(referer),
			ResponseBodyWriter: io.MultiWriter(file, hash),
		}, scopeHosts)
		closeErr := file.Close()
		if requestErr != nil {
			_ = os.Remove(downloadPath)
			if errors.Is(requestErr, errHTTPScopeViolation) || errors.Is(requestErr, errHTTPInvalidRequest) {
				return RenderImageResource{}, "scope", requestErr
			}
			if errors.Is(requestErr, errHTTPResponseTooLarge) {
				return RenderImageResource{}, "too_large", errRenderImageResourceUnavailable
			}
			if errors.Is(requestErr, context.DeadlineExceeded) || errors.Is(requestErr, context.Canceled) {
				return RenderImageResource{}, "timeout", errRenderImageResourceUnavailable
			}
			return RenderImageResource{}, "network", errRenderImageResourceUnavailable
		}
		if closeErr != nil {
			_ = os.Remove(downloadPath)
			return RenderImageResource{}, "filesystem", closeErr
		}

		if response.StatusCode >= 300 && response.StatusCode < 400 {
			_ = os.Remove(downloadPath)
			location := headerValue(response.Headers, "Location")
			nextURL, err := resolveRenderImageResourceRedirect(currentURL, location)
			if err != nil {
				return RenderImageResource{}, "redirect", err
			}
			currentURL = nextURL
			continue
		}
		if response.StatusCode != http.StatusOK || response.BodyBytes <= 0 {
			_ = os.Remove(downloadPath)
			return RenderImageResource{}, "http_status", errRenderImageResourceUnavailable
		}

		mime, extension, err := detectRenderImageResource(downloadPath)
		if err != nil {
			_ = os.Remove(downloadPath)
			return RenderImageResource{}, "unsupported_image", errRenderImageResourceUnavailable
		}
		finalPath := filepath.Join(workspace, fmt.Sprintf("resource-%02d-%02d%s", requestIndex, candidateIndex, extension))
		_ = os.Remove(finalPath)
		if err := os.Rename(downloadPath, finalPath); err != nil {
			_ = os.Remove(downloadPath)
			return RenderImageResource{}, "filesystem", err
		}
		return RenderImageResource{
			ID:     resourceID,
			Path:   finalPath,
			MIME:   mime,
			SHA256: hex.EncodeToString(hash.Sum(nil)),
			Size:   response.BodyBytes,
		}, "", nil
	}
	return RenderImageResource{}, "redirect", errRenderImageResourceUnavailable
}

func reduceRenderImageResourceSet(ctx context.Context, client *httpClient, scopeHosts []string, workspace string, items []prefetchedRenderImageResource) ([]prefetchedRenderImageResource, error) {
	total := renderImageResourceTotal(items)
	if total <= maxRenderImageResourceTotalBytes {
		return items, nil
	}
	order := make([]int, len(items))
	for index := range order {
		order[index] = index
	}
	sort.Slice(order, func(i, j int) bool {
		left := items[order[i]]
		right := items[order[j]]
		if left.resource.Size == right.resource.Size {
			return left.requestIndex < right.requestIndex
		}
		return left.resource.Size > right.resource.Size
	})
	for _, itemIndex := range order {
		for total > maxRenderImageResourceTotalBytes {
			current := items[itemIndex]
			replacement, _, err := fetchRenderImageResource(ctx, client, scopeHosts, workspace, current.requestIndex, current.spec, current.candidateIndex+1, current.resource.Size-1)
			if err != nil {
				return nil, err
			}
			if replacement == nil {
				break
			}
			_ = os.Remove(current.resource.Path)
			items[itemIndex] = *replacement
			total -= current.resource.Size - replacement.resource.Size
		}
	}
	if total > maxRenderImageResourceTotalBytes {
		return nil, errHTTPResponseTooLarge
	}
	return items, nil
}

func renderImageResourceTotal(items []prefetchedRenderImageResource) int64 {
	var total int64
	for _, item := range items {
		total += item.resource.Size
	}
	return total
}

func renderImageResourceHeaders(referer string) map[string]string {
	headers := map[string]string{
		"Accept":        "image/webp,image/png,image/jpeg,image/gif,*/*;q=0.1",
		"Cache-Control": "no-cache",
		"Pragma":        "no-cache",
		"User-Agent":    renderImageResourceUserAgent,
	}
	if strings.TrimSpace(referer) != "" {
		headers["Referer"] = referer
	}
	return headers
}

func resolveRenderImageResourceRedirect(currentURL, location string) (string, error) {
	base, err := url.Parse(currentURL)
	if err != nil || strings.TrimSpace(location) == "" {
		return "", errHTTPInvalidRequest
	}
	reference, err := url.Parse(strings.TrimSpace(location))
	if err != nil {
		return "", errHTTPInvalidRequest
	}
	resolved := base.ResolveReference(reference)
	if resolved.Scheme != "https" || resolved.Hostname() == "" || resolved.User != nil || resolved.Fragment != "" {
		return "", errHTTPInvalidRequest
	}
	return resolved.String(), nil
}

func detectRenderImageResource(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	header := make([]byte, 512)
	count, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	header = header[:count]
	mime := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(header), ";")[0]))
	if len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		mime = "image/webp"
	}
	switch mime {
	case "image/jpeg":
		return mime, ".jpg", nil
	case "image/png":
		return mime, ".png", nil
	case "image/gif":
		return mime, ".gif", nil
	case "image/webp":
		return mime, ".webp", nil
	default:
		return "", "", errRenderImageResourceUnavailable
	}
}

func headerValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func renderImageResourceFetchError(err error) error {
	switch {
	case errors.Is(err, errHTTPScopeViolation):
		return &pluginruntime.Error{Code: "plugin.capability_violation", Message: "render.image resource target is outside declared http_hosts", Err: err}
	case errors.Is(err, errHTTPInvalidRequest):
		return &pluginruntime.Error{Code: "platform.invalid_request", Message: "render.image resource request is invalid", Err: err}
	case errors.Is(err, errHTTPResponseTooLarge):
		return &pluginruntime.Error{Code: "platform.upstream_response_too_large", Message: "render.image resources exceed the request limit", Err: err}
	default:
		return &pluginruntime.Error{Code: "plugin.internal_error", Message: "render.image resource prefetch failed", Err: err}
	}
}

func logRenderImageResourceUnavailable(deps Deps, req ActionRequest, resourceID, reason string) {
	if deps.Logger == nil {
		return
	}
	deps.Logger.Warn("插件图片资源预取失败",
		"component", "render",
		"plugin_id", req.PluginID,
		"request_id", req.RequestID,
		"resource_id", strings.TrimSpace(resourceID),
		"reason", strings.TrimSpace(reason),
	)
}
