package pluginmarket

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginartifact "github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

const (
	defaultCatalogURL   = "https://raw.githubusercontent.com/RayleaBot/plugin-catalog/main/catalog.json"
	defaultSignatureURL = "https://raw.githubusercontent.com/RayleaBot/plugin-catalog/main/catalog.sig.json"
	maxCatalogBytes     = 4 * 1024 * 1024
	defaultCoreVersion  = "0.3.0"
)

// Set by official release builds with the same public-key registry used for
// release metadata. Development builds deliberately keep remote refresh
// disabled and continue to use the release-signed embedded bootstrap catalog.
var embeddedTrustedKeysSpec string

//go:embed bootstrap_catalog.json
var bootstrapFS embed.FS

type Options struct {
	CatalogURL      string
	SignatureURL    string
	TrustedKeysSpec string
	CoreVersion     string
	HTTPClient      *http.Client
	Now             func() time.Time
}

type catalogSnapshot struct {
	catalog     Catalog
	status      CatalogStatus
	digest      string
	entriesByID map[string]Entry
}

type Service struct {
	mu                 sync.RWMutex
	snapshot           catalogSnapshot
	installed          plugins.CatalogView
	installer          Installer
	options            Options
	keys               map[string]ed25519.PublicKey
	catalogValidator   *config.Validator
	signatureValidator *config.Validator
}

func New(installed plugins.CatalogView, installer Installer, options Options) (*Service, error) {
	if options.CatalogURL == "" {
		options.CatalogURL = defaultCatalogURL
	}
	if options.SignatureURL == "" {
		options.SignatureURL = defaultSignatureURL
	}
	if options.TrustedKeysSpec == "" {
		options.TrustedKeysSpec = embeddedTrustedKeysSpec
	}
	if options.CoreVersion == "" || options.CoreVersion == "0.0.0-dev" {
		options.CoreVersion = defaultCoreVersion
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	catalogValidator, err := config.CompileJSON(config.PluginStoreCatalogSchemaID, config.PluginStoreCatalogSchemaJSON)
	if err != nil {
		return nil, fmt.Errorf("compile plugin store catalog schema: %w", err)
	}
	signatureValidator, err := config.CompileJSON(config.PluginStoreSignatureSchemaID, config.PluginStoreSignatureSchemaJSON)
	if err != nil {
		return nil, fmt.Errorf("compile plugin store signature schema: %w", err)
	}
	keys, err := parseTrustedKeys(options.TrustedKeysSpec)
	if err != nil {
		return nil, fmt.Errorf("parse plugin store trusted keys: %w", err)
	}
	bootstrapBytes, err := bootstrapFS.ReadFile("bootstrap_catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded plugin store catalog: %w", err)
	}
	service := &Service{
		installed:          installed,
		installer:          installer,
		options:            options,
		keys:               keys,
		catalogValidator:   catalogValidator,
		signatureValidator: signatureValidator,
	}
	bootstrap, digest, err := service.decodeCatalog(bootstrapBytes)
	if err != nil {
		return nil, fmt.Errorf("validate embedded plugin store catalog: %w", err)
	}
	service.snapshot = newCatalogSnapshot(bootstrap, "embedded", digest, nil)
	return service, nil
}

func (s *Service) List(query Query) ListResult {
	query = normalizeQuery(query)
	snapshot := s.currentSnapshot()
	installed := installedVersions(s.installed)
	items := make([]EntryView, 0, len(snapshot.catalog.Entries))
	for _, entry := range snapshot.catalog.Entries {
		if !matchesQuery(entry, query) {
			continue
		}
		items = append(items, s.projectEntry(entry, installed[entry.ID]))
	}
	sortEntryViews(items, query.Sort)
	total := len(items)
	if query.Cursor > total {
		query.Cursor = total
	}
	end := query.Cursor + query.Limit
	if end > total {
		end = total
	}
	page := append([]EntryView(nil), items[query.Cursor:end]...)
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return ListResult{Items: page, Total: total, NextCursor: next, Catalog: snapshot.status}
}

func (s *Service) Get(pluginID string) (DetailResult, bool) {
	snapshot := s.currentSnapshot()
	entry, ok := snapshot.entriesByID[strings.TrimSpace(pluginID)]
	if !ok {
		return DetailResult{}, false
	}
	installed := installedVersions(s.installed)
	releases := append([]Release(nil), entry.Releases...)
	sort.Slice(releases, func(i, j int) bool {
		return compareSemver(releases[i].Version, releases[j].Version) > 0
	})
	views := make([]ReleaseView, 0, len(releases))
	for _, release := range releases {
		views = append(views, s.projectRelease(release))
	}
	return DetailResult{
		Plugin:   s.projectEntry(entry, installed[entry.ID]),
		Releases: views,
		Catalog:  snapshot.status,
	}, true
}

func (s *Service) Refresh(ctx context.Context) (CatalogStatus, error) {
	if len(s.keys) == 0 {
		return s.currentSnapshot().status, errorWithCode(CodeCatalogUnavailable, ErrCatalogUnavailable)
	}
	catalogBytes, err := s.fetch(ctx, s.options.CatalogURL)
	if err != nil {
		return s.currentSnapshot().status, errorWithCode(CodeCatalogUnavailable, fmt.Errorf("fetch plugin store catalog: %w", err))
	}
	signatureBytes, err := s.fetch(ctx, s.options.SignatureURL)
	if err != nil {
		return s.currentSnapshot().status, errorWithCode(CodeCatalogUnavailable, fmt.Errorf("fetch plugin store signature: %w", err))
	}
	trustedKeyIDs, err := s.verifySignature(catalogBytes, signatureBytes)
	if err != nil {
		return s.currentSnapshot().status, errorWithCode(CodeCatalogUnavailable, err)
	}
	catalog, digest, err := s.decodeCatalog(catalogBytes)
	if err != nil {
		return s.currentSnapshot().status, errorWithCode(CodeCatalogUnavailable, err)
	}
	next := newCatalogSnapshot(catalog, "remote", digest, trustedKeyIDs)
	s.mu.Lock()
	if catalogSnapshotReplays(s.snapshot, next) {
		status := s.snapshot.status
		s.mu.Unlock()
		return status, errorWithCode(CodeCatalogUnavailable, errors.New("plugin store catalog is older than the last verified snapshot"))
	}
	s.snapshot = next
	s.mu.Unlock()
	return next.status, nil
}

func (s *Service) Install(ctx context.Context, request InstallRequest) (string, error) {
	if s.installer == nil {
		return "", errorWithCode(CodeCatalogUnavailable, ErrCatalogUnavailable)
	}
	if !request.TrustedCodeConfirmed {
		return "", plugins.ErrTrustedCodeConfirmation
	}
	snapshot := s.currentSnapshot()
	entry, ok := snapshot.entriesByID[strings.TrimSpace(request.PluginID)]
	if !ok {
		return "", ErrEntryNotFound
	}
	release, asset, ok := s.resolveRelease(entry, request.Version)
	if !ok {
		return "", errorWithCode(CodeReleaseUnavailable, ErrReleaseUnavailable)
	}
	locator := "official/" + entry.ID + "@" + release.Version + "/" + asset.Platform
	installRequest := plugins.InstallRequest{
		SourceType:             "catalog",
		Source:                 locator,
		ResolvedSourceType:     "remote_url",
		ResolvedSource:         asset.URL,
		ExpectedArchiveSize:    asset.ArchiveSizeBytes,
		ExpectedArchiveSHA256:  asset.ArchiveSHA256,
		ExpectedManifestSHA256: asset.ManifestSHA256,
		ReplaceExisting:        s.pluginInstalled(entry.ID),
		PublisherID:            entry.Publisher.ID,
		PublisherName:          entry.Publisher.Name,
		PublisherVerified:      entry.Publisher.Verified,
		CatalogDigest:          snapshot.digest,
	}
	inspection, err := s.installer.Inspect(ctx, installRequest)
	if err != nil {
		return "", err
	}
	if inspection.PluginID != entry.ID || inspection.Version != release.Version || inspection.Artifact.ManifestSHA256 != asset.ManifestSHA256 {
		return "", errorWithCode(CodeIntegrityMismatch, ErrIntegrityMismatch)
	}
	installRequest.InspectionID = inspection.InspectionID
	installRequest.PackageSHA256 = inspection.PackageSHA256
	installRequest.TrustedCodeConfirmed = true
	return s.installer.Accept(ctx, installRequest)
}

func (s *Service) currentSnapshot() catalogSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneCatalogSnapshot(s.snapshot)
}

func (s *Service) projectEntry(entry Entry, installedVersion string) EntryView {
	view := EntryView{
		ID:               entry.ID,
		Name:             entry.Name,
		Summary:          entry.Summary,
		Description:      entry.Description,
		Publisher:        entry.Publisher,
		RepositoryURL:    entry.RepositoryURL,
		Homepage:         entry.Homepage,
		License:          entry.License,
		Keywords:         append([]string(nil), entry.Keywords...),
		Recommended:      entry.Recommended,
		InstalledVersion: installedVersion,
		InstallState:     "unpublished",
	}
	latest, hasLatest := latestUsableRelease(entry.Releases)
	installable, hasInstallable := s.latestInstallableRelease(entry.Releases)
	if !hasLatest && !hasInstallable {
		if catalogLatest, ok := latestCatalogRelease(entry.Releases); ok {
			releaseView := s.projectRelease(catalogLatest)
			view.LatestRelease = &releaseView
			if installedVersion != "" {
				view.InstallState = "installed"
			} else {
				view.InstallState = "incompatible"
			}
			return view
		}
		if installedVersion != "" {
			view.InstallState = "installed"
		}
		return view
	}
	if hasInstallable {
		latest = installable
		hasLatest = true
	}
	releaseView := s.projectRelease(latest)
	view.LatestRelease = &releaseView
	switch {
	case installedVersion != "" && !hasInstallable:
		view.InstallState = "installed"
	case !hasInstallable:
		view.InstallState = "incompatible"
	case installedVersion == "":
		view.InstallState = "available"
	case compareSemver(latest.Version, installedVersion) > 0:
		view.InstallState = "update_available"
	default:
		view.InstallState = "installed"
	}
	return view
}

func (s *Service) latestInstallableRelease(releases []Release) (Release, bool) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		return Release{}, false
	}
	var latest Release
	found := false
	for _, release := range releases {
		if release.Yanked || compareSemver(s.options.CoreVersion, release.MinCoreVersion) < 0 {
			continue
		}
		if _, ok := releaseAsset(release, platform); !ok {
			continue
		}
		if !found || compareSemver(release.Version, latest.Version) > 0 {
			latest = release
			found = true
		}
	}
	return latest, found
}

func (s *Service) projectRelease(release Release) ReleaseView {
	platform, _ := pluginartifact.CurrentPlatform()
	_, hasAsset := releaseAsset(release, platform)
	publishedAt, _ := time.Parse(time.RFC3339, release.PublishedAt)
	return ReleaseView{
		Version:        release.Version,
		PublishedAt:    publishedAt,
		MinCoreVersion: release.MinCoreVersion,
		Compatible:     !release.Yanked && compareSemver(s.options.CoreVersion, release.MinCoreVersion) >= 0,
		AssetAvailable: hasAsset,
		Yanked:         release.Yanked,
	}
}

func (s *Service) resolveRelease(entry Entry, requested string) (Release, Asset, bool) {
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		return Release{}, Asset{}, false
	}
	releases := append([]Release(nil), entry.Releases...)
	sort.Slice(releases, func(i, j int) bool { return compareSemver(releases[i].Version, releases[j].Version) > 0 })
	for _, release := range releases {
		if requested != "" && release.Version != requested {
			continue
		}
		if release.Yanked || compareSemver(s.options.CoreVersion, release.MinCoreVersion) < 0 {
			continue
		}
		asset, ok := releaseAsset(release, platform)
		if ok {
			return release, asset, true
		}
	}
	return Release{}, Asset{}, false
}

func (s *Service) pluginInstalled(pluginID string) bool {
	if s.installed == nil {
		return false
	}
	_, ok := s.installed.Get(pluginID)
	return ok
}

func (s *Service) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("plugin store metadata URL must use HTTPS without userinfo")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.options.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned HTTP %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, maxCatalogBytes+1)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(payload) > maxCatalogBytes {
		return nil, errors.New("plugin store metadata exceeds size limit")
	}
	return payload, nil
}

func (s *Service) decodeCatalog(payload []byte) (Catalog, string, error) {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return Catalog{}, "", invalidCatalog("decode plugin store catalog: %v", err)
	}
	if err := s.catalogValidator.Validate(document); err != nil {
		return Catalog{}, "", invalidCatalog("validate plugin store catalog schema: %v", err)
	}
	var catalog Catalog
	if err := decodeStrictJSON(payload, &catalog); err != nil {
		return Catalog{}, "", invalidCatalog("decode plugin store catalog: %v", err)
	}
	if err := validateCatalog(catalog); err != nil {
		return Catalog{}, "", invalidCatalog("validate plugin store catalog: %v", err)
	}
	digest := sha256.Sum256(payload)
	return catalog, hex.EncodeToString(digest[:]), nil
}

func (s *Service) verifySignature(catalogBytes, envelopeBytes []byte) ([]string, error) {
	var document any
	if err := json.Unmarshal(envelopeBytes, &document); err != nil {
		return nil, fmt.Errorf("decode plugin store signature: %w", err)
	}
	if err := s.signatureValidator.Validate(document); err != nil {
		return nil, fmt.Errorf("validate plugin store signature schema: %w", err)
	}
	var envelope SignatureEnvelope
	if err := decodeStrictJSON(envelopeBytes, &envelope); err != nil {
		return nil, fmt.Errorf("decode plugin store signature: %w", err)
	}
	if err := validateSignatureEnvelope(envelope); err != nil {
		return nil, fmt.Errorf("validate plugin store signature: %w", err)
	}
	digest := sha256.Sum256(catalogBytes)
	if envelope.CatalogSHA256 != hex.EncodeToString(digest[:]) {
		return nil, errors.New("plugin store signature digest does not match exact catalog bytes")
	}
	trusted := make([]string, 0, len(envelope.Signatures))
	for _, signature := range envelope.Signatures {
		key, ok := s.keys[signature.KeyID]
		if !ok {
			continue
		}
		decoded, err := base64.URLEncoding.DecodeString(signature.Signature)
		if err == nil && len(decoded) == ed25519.SignatureSize && ed25519.Verify(key, catalogBytes, decoded) {
			trusted = append(trusted, signature.KeyID)
		}
	}
	if len(trusted) == 0 {
		return nil, errors.New("no trusted key produced a valid plugin store signature")
	}
	sort.Strings(trusted)
	return trusted, nil
}

func validateSignatureEnvelope(envelope SignatureEnvelope) error {
	if envelope.SignatureVersion != 1 || envelope.Algorithm != "ed25519" {
		return errors.New("unsupported plugin store signature envelope")
	}
	seen := make(map[string]struct{}, len(envelope.Signatures))
	primaryFound := false
	for _, signature := range envelope.Signatures {
		if _, duplicate := seen[signature.KeyID]; duplicate {
			return errors.New("plugin store signature contains a duplicate key_id")
		}
		decoded, err := base64.URLEncoding.DecodeString(signature.Signature)
		if err != nil || len(decoded) != ed25519.SignatureSize {
			return errors.New("plugin store signature must contain padded base64url Ed25519 bytes")
		}
		seen[signature.KeyID] = struct{}{}
		primaryFound = primaryFound || signature.KeyID == envelope.KeyID
	}
	if !primaryFound {
		return errors.New("plugin store signature primary key_id is not present in signatures")
	}
	return nil
}

func catalogSnapshotReplays(current, next catalogSnapshot) bool {
	if next.status.GeneratedAt.Before(current.status.GeneratedAt) {
		return true
	}
	return current.status.Source == "remote" &&
		next.status.GeneratedAt.Equal(current.status.GeneratedAt) &&
		next.digest != current.digest
}

func newCatalogSnapshot(catalog Catalog, source, digest string, trustedKeyIDs []string) catalogSnapshot {
	generatedAt, _ := time.Parse(time.RFC3339, catalog.GeneratedAt)
	entries := make(map[string]Entry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		entries[entry.ID] = cloneEntry(entry)
	}
	return catalogSnapshot{
		catalog:     cloneCatalog(catalog),
		digest:      digest,
		entriesByID: entries,
		status: CatalogStatus{
			Source:        source,
			Verified:      true,
			GeneratedAt:   generatedAt,
			EntryCount:    len(catalog.Entries),
			TrustedKeyIDs: append([]string(nil), trustedKeyIDs...),
		},
	}
}

func cloneCatalogSnapshot(snapshot catalogSnapshot) catalogSnapshot {
	return newCatalogSnapshot(snapshot.catalog, snapshot.status.Source, snapshot.digest, snapshot.status.TrustedKeyIDs)
}

func cloneCatalog(catalog Catalog) Catalog {
	cloned := catalog
	cloned.Entries = make([]Entry, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		cloned.Entries = append(cloned.Entries, cloneEntry(entry))
	}
	return cloned
}

func cloneEntry(entry Entry) Entry {
	cloned := entry
	cloned.Keywords = append([]string(nil), entry.Keywords...)
	cloned.Releases = make([]Release, 0, len(entry.Releases))
	for _, release := range entry.Releases {
		copyRelease := release
		copyRelease.Assets = append([]Asset(nil), release.Assets...)
		cloned.Releases = append(cloned.Releases, copyRelease)
	}
	return cloned
}

func validateCatalog(catalog Catalog) error {
	seenPlugins := make(map[string]struct{}, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if _, exists := seenPlugins[entry.ID]; exists {
			return fmt.Errorf("duplicate plugin id %s", entry.ID)
		}
		seenPlugins[entry.ID] = struct{}{}
		seenVersions := make(map[string]struct{}, len(entry.Releases))
		for _, release := range entry.Releases {
			if _, exists := seenVersions[release.Version]; exists {
				return fmt.Errorf("plugin %s has duplicate release %s", entry.ID, release.Version)
			}
			seenVersions[release.Version] = struct{}{}
			seenPlatforms := make(map[string]struct{}, len(release.Assets))
			for _, asset := range release.Assets {
				if _, exists := seenPlatforms[asset.Platform]; exists {
					return fmt.Errorf("plugin %s release %s has duplicate platform %s", entry.ID, release.Version, asset.Platform)
				}
				seenPlatforms[asset.Platform] = struct{}{}
			}
		}
	}
	return nil
}

func parseTrustedKeys(spec string) (map[string]ed25519.PublicKey, error) {
	keys := map[string]ed25519.PublicKey{}
	for _, item := range strings.Split(strings.TrimSpace(spec), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, errors.New("trusted key entries must use key_id=base64_public_key")
		}
		keyID := strings.TrimSpace(parts[0])
		key, err := decodePublicKey(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("trusted key %s: %w", keyID, err)
		}
		keys[keyID] = key
	}
	if len(keys) > 2 {
		return nil, errors.New("plugin store supports at most two trusted keys")
	}
	return keys, nil
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
	}
	return nil, errors.New("public key must be a base64-encoded Ed25519 key")
}

func decodeStrictJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func normalizeQuery(query Query) Query {
	query.Text = strings.ToLower(strings.TrimSpace(query.Text))
	query.Publisher = strings.ToLower(strings.TrimSpace(query.Publisher))
	if query.Sort != "name" && query.Sort != "updated" {
		query.Sort = "recommended"
	}
	if query.Cursor < 0 {
		query.Cursor = 0
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 24
	}
	return query
}

func matchesQuery(entry Entry, query Query) bool {
	if query.Publisher != "" && strings.ToLower(entry.Publisher.ID) != query.Publisher {
		return false
	}
	if query.Text == "" {
		return true
	}
	values := []string{entry.ID, entry.Name, entry.Summary, entry.Description, entry.Publisher.Name, strings.Join(entry.Keywords, " ")}
	return strings.Contains(strings.ToLower(strings.Join(values, " ")), query.Text)
}

func sortEntryViews(items []EntryView, mode string) {
	sort.SliceStable(items, func(i, j int) bool {
		switch mode {
		case "name":
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		case "updated":
			left, right := items[i].LatestRelease, items[j].LatestRelease
			if left != nil && right != nil && !left.PublishedAt.Equal(right.PublishedAt) {
				return left.PublishedAt.After(right.PublishedAt)
			}
			return left != nil && right == nil
		default:
			if items[i].Recommended != items[j].Recommended {
				return items[i].Recommended
			}
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
	})
}

func installedVersions(catalog plugins.CatalogView) map[string]string {
	versions := map[string]string{}
	if catalog == nil {
		return versions
	}
	for _, snapshot := range catalog.List() {
		versions[snapshot.PluginID] = snapshot.Version
	}
	return versions
}

func latestUsableRelease(releases []Release) (Release, bool) {
	var latest Release
	found := false
	for _, release := range releases {
		if release.Yanked {
			continue
		}
		if !found || compareSemver(release.Version, latest.Version) > 0 {
			latest = release
			found = true
		}
	}
	return latest, found
}

func latestCatalogRelease(releases []Release) (Release, bool) {
	var latest Release
	found := false
	for _, release := range releases {
		if !found || compareSemver(release.Version, latest.Version) > 0 {
			latest = release
			found = true
		}
	}
	return latest, found
}

func releaseAsset(release Release, platform string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Platform == platform {
			return asset, true
		}
	}
	return Asset{}, false
}

func compareSemver(left, right string) int {
	leftParts, leftPrerelease := semverParts(left)
	rightParts, rightPrerelease := semverParts(right)
	for i := 0; i < 3; i++ {
		if leftParts[i] < rightParts[i] {
			return -1
		}
		if leftParts[i] > rightParts[i] {
			return 1
		}
	}
	return comparePrerelease(leftPrerelease, rightPrerelease)
}

func semverParts(value string) ([3]int, []string) {
	withoutBuild := strings.SplitN(value, "+", 2)[0]
	core, prerelease, hasPrerelease := strings.Cut(withoutBuild, "-")
	segments := strings.Split(core, ".")
	var parts [3]int
	for index := range parts {
		if index < len(segments) {
			parts[index], _ = strconv.Atoi(segments[index])
		}
	}
	if !hasPrerelease {
		return parts, nil
	}
	return parts, strings.Split(prerelease, ".")
}

func comparePrerelease(left, right []string) int {
	if len(left) == 0 && len(right) == 0 {
		return 0
	}
	if len(left) == 0 {
		return 1
	}
	if len(right) == 0 {
		return -1
	}
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		leftNumeric := isDecimalIdentifier(left[index])
		rightNumeric := isDecimalIdentifier(right[index])
		switch {
		case leftNumeric && rightNumeric:
			if compared := compareDecimalIdentifier(left[index], right[index]); compared != 0 {
				return compared
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			if compared := strings.Compare(left[index], right[index]); compared != 0 {
				return compared
			}
		}
	}
	return len(left) - len(right)
}

func isDecimalIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareDecimalIdentifier(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if left == "" {
		left = "0"
	}
	if right == "" {
		right = "0"
	}
	if len(left) != len(right) {
		return len(left) - len(right)
	}
	return strings.Compare(left, right)
}
