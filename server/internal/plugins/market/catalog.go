package market

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	semverutil "github.com/RayleaBot/RayleaBot/server/internal/platform/semver"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

var (
	catalogSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// Publishing validates the full schema. Readers ignore unknown fields and
// platforms while retaining the constraints on fields used by the store.
func decodeCatalog(payload []byte) (Catalog, error) {
	var catalog Catalog
	var entries []json.RawMessage
	if err := decodeCatalogFields(payload, map[string]any{
		"catalog_version": &catalog.CatalogVersion, "entries": &entries,
	}, nil); err != nil {
		return Catalog{}, invalidCatalog("decode plugin store catalog: %v", err)
	}
	if catalog.CatalogVersion != "2" {
		return Catalog{}, invalidCatalog("catalog_version must be 2")
	}
	if len(entries) > 10000 {
		return Catalog{}, invalidCatalog("catalog must contain at most 10000 entries")
	}
	catalog.Entries = make([]Entry, 0, len(entries))
	for index, data := range entries {
		entry, err := decodeCatalogEntry(data)
		if err != nil {
			return Catalog{}, invalidCatalog("catalog entry %d: %v", index, err)
		}
		catalog.Entries = append(catalog.Entries, entry)
	}
	if err := validateCatalog(catalog); err != nil {
		return Catalog{}, invalidCatalog("validate plugin store catalog: %v", err)
	}
	return catalog, nil
}

func decodeCatalogEntry(data []byte) (Entry, error) {
	var entry Entry
	var publisher, release json.RawMessage
	if err := decodeCatalogFields(data, map[string]any{
		"id": &entry.ID, "name": &entry.Name, "summary": &entry.Summary,
		"publisher": &publisher, "repository_url": &entry.RepositoryURL,
		"license": &entry.License, "keywords": &entry.Keywords, "recommended": &entry.Recommended,
	}, map[string]any{
		"description": &entry.Description, "homepage": &entry.Homepage,
		"icon_url": &entry.IconURL, "category": &entry.Category, "current_release": &release,
	}); err != nil {
		return Entry{}, err
	}
	if err := decodeCatalogFields(publisher, map[string]any{
		"id": &entry.Publisher.ID, "name": &entry.Publisher.Name,
	}, nil); err != nil {
		return Entry{}, fmt.Errorf("publisher: %w", err)
	}
	if !plugins.ValidPluginID(entry.ID) || !plugins.ValidPluginID(entry.Publisher.ID) {
		return Entry{}, errors.New("invalid plugin or publisher id")
	}
	for _, field := range []struct {
		name, value string
		limit       int
	}{
		{"name", entry.Name, 120}, {"summary", entry.Summary, 240},
		{"description", entry.Description, 4000}, {"publisher.name", entry.Publisher.Name, 120},
		{"license", entry.License, 80}, {"category", entry.Category, 64},
	} {
		if utf8.RuneCountInString(field.value) > field.limit {
			return Entry{}, fmt.Errorf("%s exceeds %d characters", field.name, field.limit)
		}
	}
	for name, value := range map[string]string{
		"repository_url": entry.RepositoryURL, "homepage": entry.Homepage, "icon_url": entry.IconURL,
	} {
		if value != "" {
			if err := validateCatalogURL(value); err != nil {
				return Entry{}, fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	if len(entry.Keywords) > 32 {
		return Entry{}, errors.New("keywords must contain at most 32 items")
	}
	seenKeywords := make(map[string]struct{}, len(entry.Keywords))
	for _, keyword := range entry.Keywords {
		if length := utf8.RuneCountInString(keyword); length < 1 || length > 64 {
			return Entry{}, errors.New("keyword must contain 1 to 64 characters")
		}
		if _, exists := seenKeywords[keyword]; exists {
			return Entry{}, fmt.Errorf("duplicate keyword %q", keyword)
		}
		seenKeywords[keyword] = struct{}{}
	}
	if release != nil {
		current, err := decodeCatalogRelease(release)
		if err != nil {
			return Entry{}, fmt.Errorf("current_release: %w", err)
		}
		entry.CurrentRelease = &current
	}
	return entry, nil
}

func decodeCatalogRelease(data []byte) (CurrentRelease, error) {
	var release CurrentRelease
	var assets []json.RawMessage
	if err := decodeCatalogFields(data, map[string]any{
		"version": &release.Version, "published_at": &release.PublishedAt,
		"min_core_version": &release.MinCoreVersion, "assets": &assets,
	}, nil); err != nil {
		return CurrentRelease{}, err
	}
	if !semverutil.Valid(release.Version) || !semverutil.Valid(release.MinCoreVersion) {
		return CurrentRelease{}, errors.New("version and min_core_version must be semantic versions")
	}
	if _, err := time.Parse(time.RFC3339, release.PublishedAt); err != nil {
		return CurrentRelease{}, fmt.Errorf("invalid published_at: %w", err)
	}
	if len(assets) == 0 {
		return CurrentRelease{}, errors.New("assets must not be empty")
	}
	release.Assets = make([]Asset, 0, 3)
	for _, data := range assets {
		var asset Asset
		if err := decodeCatalogFields(data, map[string]any{"platform": &asset.Platform}, nil); err != nil {
			return CurrentRelease{}, fmt.Errorf("asset: %w", err)
		}
		switch asset.Platform {
		case "windows-x64", "linux-x64", "macos-arm64":
		default:
			continue
		}
		if err := decodeCatalogFields(data, map[string]any{
			"url": &asset.URL, "archive_sha256": &asset.ArchiveSHA256,
		}, nil); err != nil {
			return CurrentRelease{}, fmt.Errorf("asset %s: %w", asset.Platform, err)
		}
		if !catalogSHA256Pattern.MatchString(asset.ArchiveSHA256) {
			return CurrentRelease{}, fmt.Errorf("asset %s has invalid archive_sha256", asset.Platform)
		}
		if err := validateCatalogURL(asset.URL); err != nil {
			return CurrentRelease{}, fmt.Errorf("asset %s URL: %w", asset.Platform, err)
		}
		release.Assets = append(release.Assets, asset)
	}
	// Unknown platforms do not count toward the supported asset limit.
	if len(release.Assets) > 3 {
		return CurrentRelease{}, errors.New("release must contain at most 3 supported assets")
	}
	return release, nil
}

// All consumed catalog fields reject null; all consumed strings are nonempty.
// Reading fields by name also distinguishes a missing boolean from false.
func decodeCatalogFields(data []byte, required, optional map[string]any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("expected an object")
	}
	for index, targets := range []map[string]any{required, optional} {
		for name, destination := range targets {
			value, exists := fields[name]
			if !exists {
				if index == 0 {
					return fmt.Errorf("%s is required", name)
				}
				continue
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s must not be null", name)
			}
			if err := json.Unmarshal(value, destination); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if text, ok := destination.(*string); ok && *text == "" {
				return fmt.Errorf("%s must not be empty", name)
			}
		}
	}
	return nil
}

func validateCatalogURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || !strings.HasPrefix(value, "https://") || parsed.Host == "" {
		return errors.New("URL must use HTTPS")
	}
	return nil
}
