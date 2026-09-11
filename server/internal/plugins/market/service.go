package market

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	semverutil "github.com/RayleaBot/RayleaBot/server/internal/platform/semver"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginartifact "github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

const maxCatalogBytes = 4 * 1024 * 1024

type Options struct {
	CoreVersion string
	HTTPClient  *http.Client
	Now         func() time.Time
}

type catalogSnapshot struct {
	source      Source
	catalog     Catalog
	status      SourceView
	entriesByID map[string]Entry
}

type pendingInspection struct {
	pluginID             string
	expiresAt            time.Time
	confirmationRequired bool
}

type Service struct {
	mu               sync.RWMutex
	snapshots        map[string]catalogSnapshot
	sources          map[string]Source
	pending          map[string]pendingInspection
	installed        plugins.CatalogView
	installer        Installer
	repository       Repository
	options          Options
	catalogValidator *config.Validator
}

func New(ctx context.Context, installed plugins.CatalogView, installer Installer, repository Repository, options Options) (*Service, error) {
	if repository == nil {
		return nil, errors.New("plugin store repository is required")
	}
	options.CoreVersion = strings.TrimSpace(options.CoreVersion)
	if options.CoreVersion == "" {
		return nil, errors.New("plugin store core version is required")
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	validator, err := config.CompileJSON(config.PluginStoreCatalogSchemaID, config.PluginStoreCatalogSchemaJSON)
	if err != nil {
		return nil, fmt.Errorf("compile plugin store catalog schema: %w", err)
	}
	sources, err := repository.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("plugin store repository does not contain the official source")
	}
	service := &Service{
		snapshots:        make(map[string]catalogSnapshot, len(sources)),
		sources:          make(map[string]Source, len(sources)),
		pending:          make(map[string]pendingInspection),
		installed:        installed,
		installer:        installer,
		repository:       repository,
		options:          options,
		catalogValidator: validator,
	}
	for _, source := range sources {
		service.sources[source.ID] = source
		service.snapshots[source.ID] = emptyCatalogSnapshot(source)
	}
	caches, err := repository.LoadCatalogs(ctx)
	if err != nil {
		return nil, err
	}
	for _, cached := range caches {
		source, ok := service.sources[cached.SourceID]
		if !ok {
			continue
		}
		catalog, err := service.decodeCatalog(cached.Payload)
		if err != nil {
			return nil, fmt.Errorf("load cached catalog for %s: %w", source.ID, err)
		}
		service.snapshots[source.ID] = newCatalogSnapshot(source, catalog, cached.RefreshedAt)
	}
	if _, ok := service.sources[OfficialSourceID]; !ok {
		return nil, errors.New("plugin store repository does not contain the official source")
	}
	return service, nil
}

func (s *Service) Sources() []SourceView {
	s.mu.RLock()
	items := make([]SourceView, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		items = append(items, cloneSourceView(snapshot.status))
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].Official != items[j].Official {
			return items[i].Official
		}
		if items[i].Name != items[j].Name {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *Service) CreateSource(ctx context.Context, input SourceInput) (SourceView, error) {
	name, rawURL, err := normalizeSourceInput(input)
	if err != nil {
		return SourceView{}, err
	}
	payload, catalog, err := s.fetchCatalog(ctx, rawURL)
	if err != nil {
		return SourceView{}, err
	}
	sourceID, err := newSourceID()
	if err != nil {
		return SourceView{}, err
	}
	source := Source{ID: sourceID, Name: name, URL: rawURL}
	if err := s.repository.CreateSource(ctx, source); err != nil {
		return SourceView{}, err
	}
	refreshedAt := s.options.Now().UTC()
	if err := s.repository.SaveCatalog(ctx, CachedCatalog{SourceID: source.ID, Payload: payload, RefreshedAt: refreshedAt}); err != nil {
		_ = s.repository.DeleteSource(ctx, source.ID)
		return SourceView{}, err
	}
	snapshot := newCatalogSnapshot(source, catalog, refreshedAt)
	s.mu.Lock()
	s.sources[source.ID] = source
	s.snapshots[source.ID] = snapshot
	s.mu.Unlock()
	return cloneSourceView(snapshot.status), nil
}

func (s *Service) UpdateSource(ctx context.Context, sourceID string, input SourceInput) (SourceView, error) {
	source, ok := s.source(sourceID)
	if !ok {
		return SourceView{}, ErrSourceNotFound
	}
	if source.Official {
		return SourceView{}, ErrSourceImmutable
	}
	name, rawURL, err := normalizeSourceInput(input)
	if err != nil {
		return SourceView{}, err
	}
	payload, catalog, err := s.fetchCatalog(ctx, rawURL)
	if err != nil {
		return SourceView{}, err
	}
	source.Name = name
	source.URL = rawURL
	if err := s.repository.UpdateSource(ctx, source); err != nil {
		return SourceView{}, err
	}
	refreshedAt := s.options.Now().UTC()
	if err := s.repository.SaveCatalog(ctx, CachedCatalog{SourceID: source.ID, Payload: payload, RefreshedAt: refreshedAt}); err != nil {
		return SourceView{}, err
	}
	snapshot := newCatalogSnapshot(source, catalog, refreshedAt)
	s.mu.Lock()
	s.sources[source.ID] = source
	s.snapshots[source.ID] = snapshot
	s.mu.Unlock()
	return cloneSourceView(snapshot.status), nil
}

func (s *Service) DeleteSource(ctx context.Context, sourceID string) error {
	source, ok := s.source(sourceID)
	if !ok {
		return ErrSourceNotFound
	}
	if source.Official {
		return ErrSourceImmutable
	}
	if err := s.repository.DeleteSource(ctx, source.ID); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.sources, source.ID)
	delete(s.snapshots, source.ID)
	s.mu.Unlock()
	return nil
}

func (s *Service) List(query Query) (ListResult, error) {
	query = normalizeQuery(query)
	snapshot, ok := s.snapshot(query.SourceID)
	if !ok {
		return ListResult{}, ErrSourceNotFound
	}
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
	end := min(query.Cursor+query.Limit, total)
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return ListResult{
		Items:      append([]EntryView(nil), items[query.Cursor:end]...),
		Total:      total,
		NextCursor: next,
		Source:     cloneSourceView(snapshot.status),
	}, nil
}

func (s *Service) Get(sourceID, pluginID string) (DetailResult, bool) {
	if strings.TrimSpace(sourceID) == "" {
		sourceID = OfficialSourceID
	}
	snapshot, ok := s.snapshot(sourceID)
	if !ok {
		return DetailResult{}, false
	}
	entry, ok := snapshot.entriesByID[strings.TrimSpace(pluginID)]
	if !ok {
		return DetailResult{}, false
	}
	installed := installedVersions(s.installed)
	view := s.projectEntry(entry, installed[entry.ID])
	var currentRelease *ReleaseView
	if entry.CurrentRelease != nil {
		release := s.projectRelease(*entry.CurrentRelease)
		currentRelease = &release
	}
	return DetailResult{Plugin: view, CurrentRelease: currentRelease, Source: cloneSourceView(snapshot.status)}, true
}

func (s *Service) Refresh(ctx context.Context, sourceID string) (SourceView, error) {
	source, ok := s.source(sourceID)
	if !ok {
		return SourceView{}, ErrSourceNotFound
	}
	payload, catalog, err := s.fetchCatalog(ctx, source.URL)
	if err != nil {
		return SourceView{}, err
	}
	refreshedAt := s.options.Now().UTC()
	if err := s.repository.SaveCatalog(ctx, CachedCatalog{SourceID: source.ID, Payload: payload, RefreshedAt: refreshedAt}); err != nil {
		return SourceView{}, err
	}
	snapshot := newCatalogSnapshot(source, catalog, refreshedAt)
	s.mu.Lock()
	s.snapshots[source.ID] = snapshot
	s.mu.Unlock()
	return cloneSourceView(snapshot.status), nil
}

func (s *Service) Inspect(ctx context.Context, request InspectionRequest) (InspectionResult, error) {
	if s.installer == nil {
		return InspectionResult{}, errorWithCode(CodeCatalogUnavailable, ErrCatalogUnavailable)
	}
	snapshot, ok := s.snapshot(request.SourceID)
	if !ok {
		return InspectionResult{}, ErrSourceNotFound
	}
	entry, ok := snapshot.entriesByID[strings.TrimSpace(request.PluginID)]
	if !ok {
		return InspectionResult{}, ErrEntryNotFound
	}
	release, asset, ok := s.resolveRelease(entry)
	if !ok {
		return InspectionResult{}, errorWithCode(CodeReleaseUnavailable, ErrReleaseUnavailable)
	}
	installRequest := plugins.InstallRequest{
		SourceType:            "catalog",
		Source:                snapshot.source.ID,
		SourceLabel:           snapshot.source.Name,
		ResolvedSourceType:    "remote_url",
		ResolvedSource:        asset.URL,
		ExpectedArchiveSHA256: asset.ArchiveSHA256,
		ReplaceExisting:       s.pluginInstalled(entry.ID),
		TrustedCodeRequired:   true,
	}
	inspection, err := s.installer.Inspect(ctx, installRequest)
	if err != nil {
		return InspectionResult{}, err
	}
	if inspection.PluginID != entry.ID || inspection.Version != release.Version {
		return InspectionResult{}, errorWithCode(CodeIntegrityMismatch, ErrIntegrityMismatch)
	}
	reasons := s.confirmationReasons(snapshot.source.ID, inspection)
	result := InspectionResult{
		Inspection:           inspection,
		ConfirmationRequired: len(reasons) > 0,
		ConfirmationReasons:  reasons,
	}
	s.mu.Lock()
	s.cleanupPendingLocked(s.options.Now().UTC())
	s.pending[inspection.InspectionID] = pendingInspection{
		pluginID:             entry.ID,
		expiresAt:            inspection.ExpiresAt,
		confirmationRequired: result.ConfirmationRequired,
	}
	s.mu.Unlock()
	return result, nil
}

func (s *Service) Install(ctx context.Context, request InstallRequest) (string, error) {
	s.mu.Lock()
	s.cleanupPendingLocked(s.options.Now().UTC())
	pending, ok := s.pending[strings.TrimSpace(request.InspectionID)]
	if !ok {
		s.mu.Unlock()
		return "", plugins.ErrInstallInspectionRequired
	}
	if pending.pluginID != strings.TrimSpace(request.PluginID) {
		s.mu.Unlock()
		return "", plugins.ErrInstallDigestMismatch
	}
	if pending.confirmationRequired && !request.TrustedCodeConfirmed {
		s.mu.Unlock()
		return "", plugins.ErrTrustedCodeConfirmation
	}
	s.mu.Unlock()
	taskID, err := s.installer.Accept(ctx, plugins.InstallAcceptance{
		InspectionID:         request.InspectionID,
		PackageSHA256:        request.PackageSHA256,
		TrustedCodeConfirmed: true,
	})
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	delete(s.pending, request.InspectionID)
	s.mu.Unlock()
	return taskID, nil
}

func (s *Service) confirmationReasons(sourceID string, inspection plugins.InstallInspection) []string {
	if s.installed == nil {
		return []string{"first_install"}
	}
	current, ok := s.installed.Get(inspection.PluginID)
	if !ok {
		return []string{"first_install"}
	}
	reasons := make([]string, 0, 2)
	if current.PackageSourceType != "catalog" || current.PackageSourceRef != sourceID {
		reasons = append(reasons, "source_changed")
	}
	if permissionsExpanded(current.Permissions, inspection.Permissions) {
		reasons = append(reasons, "permissions_expanded")
	}
	return reasons
}

func permissionsExpanded(current, next map[string]plugins.PermissionGrant) bool {
	for name, nextGrant := range next {
		currentGrant, ok := current[name]
		if !ok {
			return true
		}
		if len(currentGrant.Platforms) == 0 {
			continue
		}
		if len(nextGrant.Platforms) == 0 {
			return true
		}
		allowed := make(map[string]struct{}, len(currentGrant.Platforms))
		for _, platform := range currentGrant.Platforms {
			allowed[platform] = struct{}{}
		}
		for _, platform := range nextGrant.Platforms {
			if _, ok := allowed[platform]; !ok {
				return true
			}
		}
	}
	return false
}

func (s *Service) cleanupPendingLocked(now time.Time) {
	for id, pending := range s.pending {
		if !pending.expiresAt.After(now) {
			delete(s.pending, id)
		}
	}
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
		IconURL:          entry.IconURL,
		License:          entry.License,
		Keywords:         append([]string(nil), entry.Keywords...),
		Recommended:      entry.Recommended,
		Category:         entry.Category,
		InstalledVersion: installedVersion,
		InstallState:     "unpublished",
	}
	if entry.CurrentRelease == nil {
		if installedVersion != "" {
			view.InstallState = "installed"
		}
		return view
	}
	releaseView := s.projectRelease(*entry.CurrentRelease)
	view.LatestRelease = &releaseView
	switch {
	case installedVersion != "" && (!releaseView.Compatible || !releaseView.AssetAvailable):
		view.InstallState = "installed"
	case !releaseView.Compatible || !releaseView.AssetAvailable:
		view.InstallState = "incompatible"
	case installedVersion == "":
		view.InstallState = "available"
	case semverutil.Compare(entry.CurrentRelease.Version, installedVersion) > 0:
		view.InstallState = "update_available"
	default:
		view.InstallState = "installed"
	}
	return view
}

func (s *Service) projectRelease(release CurrentRelease) ReleaseView {
	platform, _ := pluginartifact.CurrentPlatform()
	_, hasAsset := releaseAsset(release, platform)
	publishedAt, _ := time.Parse(time.RFC3339, release.PublishedAt)
	return ReleaseView{
		Version:        release.Version,
		PublishedAt:    publishedAt,
		MinCoreVersion: release.MinCoreVersion,
		Compatible:     s.options.CoreVersion != "unknown" && semverutil.Compare(s.options.CoreVersion, release.MinCoreVersion) >= 0,
		AssetAvailable: hasAsset,
	}
}

func (s *Service) resolveRelease(entry Entry) (CurrentRelease, Asset, bool) {
	if entry.CurrentRelease == nil {
		return CurrentRelease{}, Asset{}, false
	}
	if s.options.CoreVersion == "unknown" || semverutil.Compare(s.options.CoreVersion, entry.CurrentRelease.MinCoreVersion) < 0 {
		return CurrentRelease{}, Asset{}, false
	}
	platform, err := pluginartifact.CurrentPlatform()
	if err != nil {
		return CurrentRelease{}, Asset{}, false
	}
	asset, ok := releaseAsset(*entry.CurrentRelease, platform)
	return *entry.CurrentRelease, asset, ok
}

func (s *Service) pluginInstalled(pluginID string) bool {
	if s.installed == nil {
		return false
	}
	_, ok := s.installed.Get(pluginID)
	return ok
}

func (s *Service) source(sourceID string) (Source, bool) {
	if strings.TrimSpace(sourceID) == "" {
		sourceID = OfficialSourceID
	}
	s.mu.RLock()
	source, ok := s.sources[sourceID]
	s.mu.RUnlock()
	return source, ok
}

func (s *Service) snapshot(sourceID string) (catalogSnapshot, bool) {
	if strings.TrimSpace(sourceID) == "" {
		sourceID = OfficialSourceID
	}
	s.mu.RLock()
	snapshot, ok := s.snapshots[sourceID]
	s.mu.RUnlock()
	if !ok {
		return catalogSnapshot{}, false
	}
	return cloneCatalogSnapshot(snapshot), true
}

func (s *Service) fetchCatalog(ctx context.Context, rawURL string) ([]byte, Catalog, error) {
	payload, err := s.fetch(ctx, rawURL)
	if err != nil {
		return nil, Catalog{}, errorWithCode(CodeCatalogUnavailable, fmt.Errorf("fetch plugin store catalog: %w", err))
	}
	catalog, err := s.decodeCatalog(payload)
	if err != nil {
		return nil, Catalog{}, err
	}
	return payload, catalog, nil
}

func (s *Service) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateSourceURL(rawURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	client := *s.options.HTTPClient
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("plugin store redirect limit exceeded")
		}
		return validateSourceURL(request.URL.String())
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote server returned HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxCatalogBytes {
		return nil, errors.New("plugin store catalog exceeds size limit")
	}
	return payload, nil
}

func (s *Service) decodeCatalog(payload []byte) (Catalog, error) {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return Catalog{}, invalidCatalog("decode plugin store catalog: %v", err)
	}
	if err := s.catalogValidator.Validate(document); err != nil {
		return Catalog{}, invalidCatalog("validate plugin store catalog schema: %v", err)
	}
	var catalog Catalog
	if err := decodeStrictJSON(payload, &catalog); err != nil {
		return Catalog{}, invalidCatalog("decode plugin store catalog: %v", err)
	}
	if err := validateCatalog(catalog); err != nil {
		return Catalog{}, invalidCatalog("validate plugin store catalog: %v", err)
	}
	return catalog, nil
}

func validateCatalog(catalog Catalog) error {
	seenPlugins := make(map[string]struct{}, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if _, exists := seenPlugins[entry.ID]; exists {
			return fmt.Errorf("duplicate plugin id %s", entry.ID)
		}
		seenPlugins[entry.ID] = struct{}{}
		if entry.CurrentRelease == nil {
			continue
		}
		seenPlatforms := make(map[string]struct{}, len(entry.CurrentRelease.Assets))
		for _, asset := range entry.CurrentRelease.Assets {
			if err := validateSourceURL(asset.URL); err != nil {
				return fmt.Errorf("plugin %s has invalid asset URL: %w", entry.ID, err)
			}
			if _, exists := seenPlatforms[asset.Platform]; exists {
				return fmt.Errorf("plugin %s has duplicate platform %s", entry.ID, asset.Platform)
			}
			seenPlatforms[asset.Platform] = struct{}{}
		}
	}
	return nil
}

func normalizeSourceInput(input SourceInput) (string, string, error) {
	name := strings.TrimSpace(input.Name)
	rawURL := strings.TrimSpace(input.URL)
	if name == "" || len([]rune(name)) > 120 {
		return "", "", fmt.Errorf("%w: name must contain 1 to 120 characters", ErrSourceInvalid)
	}
	if err := validateSourceURL(rawURL); err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrSourceInvalid, err)
	}
	return name, rawURL, nil
}

func validateSourceURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("plugin store source must use HTTPS without userinfo")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("plugin store source must not use a local host")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()) {
		return errors.New("plugin store source must not use a local or private address")
	}
	return nil
}

func newSourceID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "source-" + hex.EncodeToString(buffer), nil
}

func emptyCatalogSnapshot(source Source) catalogSnapshot {
	return catalogSnapshot{
		source:      source,
		catalog:     Catalog{CatalogVersion: "2", Entries: []Entry{}},
		status:      SourceView{ID: source.ID, Name: source.Name, URL: source.URL, Official: source.Official, Cached: false, EntryCount: 0},
		entriesByID: map[string]Entry{},
	}
}

func newCatalogSnapshot(source Source, catalog Catalog, refreshedAt time.Time) catalogSnapshot {
	entries := make(map[string]Entry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		entries[entry.ID] = cloneEntry(entry)
	}
	timeCopy := refreshedAt.UTC()
	return catalogSnapshot{
		source:      source,
		catalog:     cloneCatalog(catalog),
		status:      SourceView{ID: source.ID, Name: source.Name, URL: source.URL, Official: source.Official, Cached: true, RefreshedAt: &timeCopy, EntryCount: len(catalog.Entries)},
		entriesByID: entries,
	}
}

func cloneCatalogSnapshot(snapshot catalogSnapshot) catalogSnapshot {
	cloned := snapshot
	cloned.catalog = cloneCatalog(snapshot.catalog)
	cloned.status = cloneSourceView(snapshot.status)
	cloned.entriesByID = make(map[string]Entry, len(snapshot.entriesByID))
	for id, entry := range snapshot.entriesByID {
		cloned.entriesByID[id] = cloneEntry(entry)
	}
	return cloned
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
	if entry.CurrentRelease != nil {
		release := *entry.CurrentRelease
		release.Assets = append([]Asset(nil), entry.CurrentRelease.Assets...)
		cloned.CurrentRelease = &release
	}
	return cloned
}

func cloneSourceView(view SourceView) SourceView {
	cloned := view
	if view.RefreshedAt != nil {
		value := *view.RefreshedAt
		cloned.RefreshedAt = &value
	}
	return cloned
}

func normalizeQuery(query Query) Query {
	query.SourceID = strings.TrimSpace(query.SourceID)
	if query.SourceID == "" {
		query.SourceID = OfficialSourceID
	}
	query.Text = strings.ToLower(strings.TrimSpace(query.Text))
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
	if query.Text == "" {
		return true
	}
	values := []string{entry.ID, entry.Name, entry.Summary, entry.Description, entry.Publisher.Name, entry.Category}
	values = append(values, entry.Keywords...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query.Text) {
			return true
		}
	}
	return false
}

func sortEntryViews(items []EntryView, mode string) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		switch mode {
		case "name":
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
		case "updated":
			if left.LatestRelease != nil && right.LatestRelease != nil {
				return left.LatestRelease.PublishedAt.After(right.LatestRelease.PublishedAt)
			}
			return left.LatestRelease != nil && right.LatestRelease == nil
		default:
			if left.Recommended != right.Recommended {
				return left.Recommended
			}
			return strings.ToLower(left.Name) < strings.ToLower(right.Name)
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

func releaseAsset(release CurrentRelease, platform string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Platform == platform {
			return asset, true
		}
	}
	return Asset{}, false
}

func decodeStrictJSON(payload []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("JSON must contain exactly one value")
		}
		return err
	}
	return nil
}
