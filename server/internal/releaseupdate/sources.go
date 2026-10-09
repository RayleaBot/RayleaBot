package releaseupdate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const releasesAPI = "https://api.github.com/repos/RayleaBot/RayleaBot/releases?per_page=100"

type fetched struct {
	Route
	payload []byte
	err     error
}

func (c *Checker) candidates(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	github := u.Hostname() == "github.com" || u.Hostname() == "api.github.com" || u.Hostname() == "raw.githubusercontent.com"
	if !github {
		return []string{raw}
	}
	var result []string
	if c.Settings.Mode != "proxy" {
		result = append(result, raw)
	}
	if c.Settings.Mode != "direct" {
		for _, prefix := range c.Settings.Proxies {
			result = append(result, strings.TrimRight(prefix, "/")+"/"+raw)
		}
	}
	return uniqueURLs(result)
}

func uniqueURLs(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (c *Checker) fetchMany(ctx context.Context, urls []string) []fetched {
	urls = uniqueURLs(urls)
	results := make([]fetched, len(urls))
	var wg sync.WaitGroup
	workers := make(chan struct{}, 4)
	for i, raw := range urls {
		wg.Go(func() {
			results[i].URL = raw
			select {
			case workers <- struct{}{}:
			case <-ctx.Done():
				results[i].err = ctx.Err()
				return
			}
			defer func() { <-workers }()
			started := time.Now()
			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			results[i].payload, results[i].err = c.fetchMetadata(requestCtx, raw)
			results[i].LatencyMS = time.Since(started).Milliseconds()
		})
	}
	wg.Wait()
	return results
}

func (c *Checker) Releases(ctx context.Context) ([]Release, error) {
	c = c.configured()
	urls := c.candidates(releasesAPI)
	for _, base := range c.Settings.Mirrors {
		urls = append(urls, base+"/releases.json")
	}
	byVersion := map[string]Release{}
	valid := false
	for _, response := range c.fetchMany(ctx, urls) {
		if response.err != nil {
			continue
		}
		var entries []struct {
			Tag        string `json:"tag_name"`
			Version    string `json:"version"`
			Draft      bool   `json:"draft"`
			Prerelease bool   `json:"prerelease"`
			Channel    string `json:"channel"`
			Page       string `json:"html_url"`
			Notes      string `json:"release_notes_ref"`
		}
		if json.Unmarshal(response.payload, &entries) != nil || entries == nil {
			continue
		}
		valid = true
		for _, entry := range entries {
			version := entry.Version
			if version == "" {
				version = strings.TrimPrefix(entry.Tag, "v")
			}
			parsed, err := parseSemanticVersion(version)
			if err != nil || entry.Draft {
				continue
			}
			beta := entry.Prerelease || entry.Channel == "beta" || len(parsed.prerelease) > 0
			if c.Settings.Channel != "beta" && beta {
				continue
			}
			page := entry.Notes
			if page == "" {
				page = entry.Page
			}
			if validateHTTPSURL(page) != nil {
				continue
			}
			channel := "stable"
			if beta {
				channel = "beta"
			}
			byVersion[version] = Release{Version: version, Channel: channel, ReleaseNotesRef: page}
		}
	}
	if !valid {
		return nil, errorWithCode(CodeManifestInvalid, "list releases", errors.New("no reachable release list"))
	}
	result := make([]Release, 0, len(byVersion))
	for _, entry := range byVersion {
		result = append(result, entry)
	}
	slices.SortFunc(result, func(a, b Release) int { cmp, _ := compareSemanticVersions(b.Version, a.Version); return cmp })
	if len(result) > 30 {
		result = result[:30]
	}
	return result, nil
}

func (c *Checker) selectManifest(ctx context.Context, artifactID string) (Manifest, []Route, string, error) {
	version := c.Settings.Version
	betaListUnavailable := false
	if version == "" && c.Settings.Channel == "beta" {
		if releases, err := c.Releases(ctx); err == nil && len(releases) > 0 {
			version = releases[0].Version
		} else {
			betaListUnavailable = true
		}
	}
	raw := c.ManifestURL
	if version != "" {
		raw = ReleaseRepositoryURL + "/releases/download/v" + url.PathEscape(version) + "/" + ManifestAssetName
	}
	urls := c.candidates(raw)
	if betaListUnavailable {
		urls = nil
	}
	for _, base := range c.Settings.Mirrors {
		path := "/" + c.Settings.Channel + ".json"
		if c.Settings.Version != "" {
			path = "/v" + url.PathEscape(version) + "/" + ManifestAssetName
		}
		urls = append(urls, base+path)
	}
	var best Manifest
	selected := -1
	responses := c.fetchMany(ctx, urls)
	routes := make([]Route, len(responses))
	for i, response := range responses {
		routes[i] = response.Route
		if response.err != nil {
			continue
		}
		manifest, err := decodeManifest(response.payload)
		if err != nil {
			continue
		}
		if _, err := selectArtifact(manifest, artifactID); err != nil {
			continue
		}
		parsed, _ := parseSemanticVersion(manifest.Version)
		if c.Settings.Channel != "beta" && len(parsed.prerelease) > 0 {
			continue
		}
		if c.Settings.Version != "" && manifest.Version != c.Settings.Version {
			continue
		}
		routes[i].Available = true
		cmp := 1
		if selected >= 0 {
			cmp, _ = compareSemanticVersions(manifest.Version, best.Version)
		}
		if cmp > 0 || (cmp == 0 && routes[i].LatencyMS < routes[selected].LatencyMS) {
			best = manifest
			selected = i
		}
	}
	if selected < 0 {
		return Manifest{}, routes, "", errors.New("no valid release metadata from configured routes")
	}
	routes[selected].Selected = true
	return best, routes, routes[selected].URL, nil
}

// Probe an actual bounded archive sample, not a proxy landing page or a HEAD
// response. A server ignoring Range still costs at most 64 KiB before closing.
func (c *Checker) downloadRoutes(ctx context.Context, artifact Artifact, version string) []string {
	urls := c.candidates(artifact.DownloadURL)
	if len(c.Settings.Mirrors) > 0 {
		urls = append(urls, c.candidates(ReleaseRepositoryURL+"/releases/download/v"+url.PathEscape(version)+"/"+artifact.FileName)...)
	}
	for _, base := range c.Settings.Mirrors {
		urls = append(urls, base+"/v"+url.PathEscape(version)+"/"+artifact.FileName)
	}
	urls = uniqueURLs(urls)
	if len(urls) < 2 {
		return urls
	}
	type sample struct {
		url     string
		elapsed time.Duration
		valid   bool
	}
	samples := make([]sample, len(urls))
	var wg sync.WaitGroup
	workers := make(chan struct{}, 4)
	for i, raw := range urls {
		wg.Go(func() {
			samples[i].url = raw
			select {
			case workers <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-workers }()
			probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, raw, nil)
			if err != nil || validateHTTPSURL(raw) != nil {
				return
			}
			request.Header.Set("Range", "bytes=0-65535")
			request.Header.Set("User-Agent", "RayleaBot-UpdateCheck/2")
			client := c.DownloadClient
			if client == nil {
				client = newSecureHTTPClient(0)
			}
			start := time.Now()
			response, err := client.Do(request)
			if err != nil {
				return
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
				return
			}
			bytes, err := io.ReadAll(io.LimitReader(response.Body, 65536))
			if err != nil || len(bytes) < 2 {
				return
			}
			isZip := strings.HasSuffix(artifact.FileName, ".zip")
			if isZip && (len(bytes) < 4 || string(bytes[:4]) != "PK\x03\x04") {
				return
			}
			if !isZip && (bytes[0] != 0x1f || bytes[1] != 0x8b) {
				return
			}
			samples[i].valid = true
			samples[i].elapsed = time.Since(start)
		})
	}
	wg.Wait()
	slices.SortStableFunc(samples, func(a, b sample) int {
		if a.valid != b.valid {
			if a.valid {
				return -1
			}
			return 1
		}
		return int(a.elapsed - b.elapsed)
	})
	result := make([]string, 0, len(samples))
	for _, sample := range samples {
		result = append(result, sample.url)
	}
	return result
}

func (c *Checker) report(stage, raw string, done, total int64) {
	if c.Progress != nil {
		c.Progress(Progress{Stage: stage, SourceURL: raw, DownloadedBytes: done, TotalBytes: total})
	}
}
