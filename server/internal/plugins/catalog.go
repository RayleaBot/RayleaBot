package plugins

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrPluginNotFound = errors.New("plugin not found")
	ErrStateConflict  = errors.New("state conflict")
)

type PermissionPendingError struct {
	PluginID            string
	MissingCapabilities []string
	ScopeChanged        bool
}

func (e *PermissionPendingError) Error() string {
	return "plugin permission pending"
}

type Command struct {
	Name          string
	Aliases       []string
	Description   string
	Usage         string
	Permission    string
	CommandSource string
	DeclarationID string
}

type DynamicCommandDecl struct {
	ID          string
	SettingsKey string
	Description string
	UsageArgs   string
	Permission  string
}

type WebhookScope struct {
	Route           string   `json:"route"`
	AuthStrategy    string   `json:"auth_strategy"`
	Header          string   `json:"header"`
	SecretRef       string   `json:"secret_ref"`
	SignaturePrefix string   `json:"signature_prefix,omitempty"`
	SourceIPs       []string `json:"source_ips,omitempty"`
}

type Screenshot struct {
	Path string `json:"path"`
	Alt  string `json:"alt,omitempty"`
}

type ManagementUI struct {
	Entry string `json:"entry"`
	Label string `json:"label,omitempty"`
}

type RenderTemplate struct {
	Path string `json:"path"`
}

type Snapshot struct {
	PluginID              string
	Name                  string
	Role                  string
	Version               string
	Author                string
	License               string
	SDKMinVersion         string
	RuntimeVersion        string
	MinCoreVersion        string
	DataSchemaVersion     string
	Concurrency           int
	Platforms             []string
	Runtime               string
	Entry                 string
	Type                  string
	Description           string
	Icon                  string
	Repo                  string
	Homepage              string
	Keywords              []string
	Screenshots           []Screenshot
	ManagementUI          *ManagementUI
	RenderTemplates       []RenderTemplate
	SystemDependencies    []string
	DefaultConfig         map[string]any
	ManifestPath          string
	PackageRootPath       string
	SourceRoot            string
	SourceRoots           []string
	PackageSourceType     string
	PackageSourceRef      string
	Valid                 bool
	ValidationSummary     string
	RegistrationState     string
	DesiredState          string
	RuntimeState          string
	DisplayState          string
	ConflictPaths         []string
	RequiredPermissions   []string
	OptionalPermissions   []string
	DeclaredCapabilities  []string
	PythonDependencies    []string
	NodeDependencies      []string
	RequireInstallScripts bool
	ScopeHTTPHosts        []string
	ScopeStorageRoots     []string
	ScopeWebhooks         []WebhookScope
	Commands              []Command
	ManifestCommands      []Command
	DynamicCommands       []DynamicCommandDecl
}

type Catalog struct {
	mu          sync.RWMutex
	order       []string
	items       map[string]Snapshot
	nextSubID   uint64
	subscribers map[uint64]chan Snapshot
}

func NewCatalog(entries []Snapshot) *Catalog {
	items := make(map[string]Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		items[entry.PluginID] = cloneSnapshot(entry)
		if _, ok := seen[entry.PluginID]; ok {
			continue
		}
		seen[entry.PluginID] = struct{}{}
		order = append(order, entry.PluginID)
	}

	sort.Strings(order)

	return &Catalog{
		order:       order,
		items:       items,
		subscribers: make(map[uint64]chan Snapshot),
	}
}

func (c *Catalog) List() []Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, cloneSnapshot(c.items[pluginID]))
	}

	return result
}

func (c *Catalog) Get(pluginID string) (Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, false
	}

	return cloneSnapshot(entry), true
}

func (c *Catalog) SetDesiredState(pluginID string, desired string) (Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, ErrPluginNotFound
	}

	if entry.RegistrationState != "installed" {
		c.mu.Unlock()
		return Snapshot{}, ErrStateConflict
	}

	if entry.DesiredState == desired {
		c.mu.Unlock()
		return Snapshot{}, ErrStateConflict
	}

	entry.DesiredState = desired
	entry.DisplayState = defaultDisplayState(entry)
	changed := pluginStateChanged(c.items[pluginID], entry)
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	c.mu.Unlock()

	if changed {
		c.publish(updated)
	}
	return updated, nil
}

func (c *Catalog) SetRuntimeState(pluginID string, runtimeState string) (Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, ErrPluginNotFound
	}

	current := entry
	entry.RuntimeState = runtimeState
	entry.DisplayState = defaultDisplayState(entry)
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	c.mu.Unlock()

	if pluginStateChanged(current, entry) {
		c.publish(updated)
	}
	return updated, nil
}

func (c *Catalog) ApplyDesiredStates(states map[string]string) {
	if len(states) == 0 {
		return
	}

	c.mu.Lock()
	updated := make([]Snapshot, 0, len(states))

	for pluginID, desired := range states {
		entry, ok := c.items[pluginID]
		if !ok {
			continue
		}
		if entry.RegistrationState != "installed" {
			continue
		}
		if desired != "enabled" && desired != "disabled" {
			continue
		}

		current := entry
		entry.DesiredState = desired
		entry.DisplayState = defaultDisplayState(entry)
		c.items[pluginID] = entry
		if pluginStateChanged(current, entry) {
			updated = append(updated, cloneSnapshot(entry))
		}
	}
	c.mu.Unlock()

	c.publishMany(updated)
}

func (c *Catalog) Replace(entries []Snapshot) {
	c.mu.Lock()

	items := make(map[string]Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	updated := make([]Snapshot, 0, len(entries))

	for _, entry := range entries {
		cloned := cloneSnapshot(entry)
		items[entry.PluginID] = cloned
		if current, ok := c.items[entry.PluginID]; !ok || pluginStateChanged(current, cloned) {
			updated = append(updated, cloned)
		}
		if _, ok := seen[entry.PluginID]; ok {
			continue
		}
		seen[entry.PluginID] = struct{}{}
		order = append(order, entry.PluginID)
	}

	sort.Strings(order)
	c.items = items
	c.order = order
	c.mu.Unlock()

	c.publishMany(updated)
}

func (c *Catalog) RefreshCommands(pluginID string, settings map[string]any) (Snapshot, bool) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return Snapshot{}, false
	}

	current := entry
	entry.Commands = ProjectCommands(entry, settings)
	changed := pluginStateChanged(current, entry)
	c.items[pluginID] = entry
	updated := cloneSnapshot(entry)
	published := []Snapshot{updated}
	if changed {
		published = make([]Snapshot, 0, len(c.order))
		for _, id := range c.order {
			published = append(published, cloneSnapshot(c.items[id]))
		}
	}
	c.mu.Unlock()

	if changed {
		c.publishMany(published)
	}
	return updated, true
}

func (c *Catalog) Subscribe(buffer int) (<-chan Snapshot, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Snapshot, buffer)
	c.mu.Lock()
	id := c.nextSubID
	c.nextSubID++
	c.subscribers[id] = ch
	c.mu.Unlock()

	return ch, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		subscriber, ok := c.subscribers[id]
		if !ok {
			return
		}
		delete(c.subscribers, id)
		close(subscriber)
	}
}

func (c *Catalog) SubscriberCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.subscribers)
}

func (c *Catalog) publish(snapshot Snapshot) {
	c.publishMany([]Snapshot{snapshot})
}

func (c *Catalog) publishMany(snapshots []Snapshot) {
	if len(snapshots) == 0 {
		return
	}

	c.mu.RLock()
	subscribers := make([]chan Snapshot, 0, len(c.subscribers))
	for _, subscriber := range c.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	c.mu.RUnlock()

	for _, snapshot := range snapshots {
		for _, subscriber := range subscribers {
			select {
			case subscriber <- cloneSnapshot(snapshot):
			default:
			}
		}
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.DisplayState = projectDisplayState(snapshot)
	cloned.DefaultConfig = cloneMap(snapshot.DefaultConfig)
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	cloned.Platforms = append([]string(nil), snapshot.Platforms...)
	cloned.Keywords = append([]string(nil), snapshot.Keywords...)
	cloned.SystemDependencies = append([]string(nil), snapshot.SystemDependencies...)
	cloned.RequiredPermissions = append([]string(nil), snapshot.RequiredPermissions...)
	cloned.OptionalPermissions = append([]string(nil), snapshot.OptionalPermissions...)
	cloned.DeclaredCapabilities = append([]string(nil), snapshot.DeclaredCapabilities...)
	cloned.PythonDependencies = append([]string(nil), snapshot.PythonDependencies...)
	cloned.NodeDependencies = append([]string(nil), snapshot.NodeDependencies...)
	cloned.ScopeHTTPHosts = append([]string(nil), snapshot.ScopeHTTPHosts...)
	cloned.ScopeStorageRoots = append([]string(nil), snapshot.ScopeStorageRoots...)
	if len(snapshot.ScopeWebhooks) > 0 {
		cloned.ScopeWebhooks = make([]WebhookScope, 0, len(snapshot.ScopeWebhooks))
		for _, scope := range snapshot.ScopeWebhooks {
			copied := scope
			copied.SourceIPs = append([]string(nil), scope.SourceIPs...)
			cloned.ScopeWebhooks = append(cloned.ScopeWebhooks, copied)
		}
	}
	if len(snapshot.Screenshots) > 0 {
		cloned.Screenshots = make([]Screenshot, 0, len(snapshot.Screenshots))
		for _, screenshot := range snapshot.Screenshots {
			cloned.Screenshots = append(cloned.Screenshots, screenshot)
		}
	}
	if snapshot.ManagementUI != nil {
		copied := *snapshot.ManagementUI
		cloned.ManagementUI = &copied
	}
	if len(snapshot.RenderTemplates) > 0 {
		cloned.RenderTemplates = append([]RenderTemplate(nil), snapshot.RenderTemplates...)
	}
	if len(snapshot.Commands) > 0 {
		cloned.Commands = cloneCommands(snapshot.Commands)
	}
	if len(snapshot.ManifestCommands) > 0 {
		cloned.ManifestCommands = cloneCommands(snapshot.ManifestCommands)
	}
	if len(snapshot.DynamicCommands) > 0 {
		cloned.DynamicCommands = append([]DynamicCommandDecl(nil), snapshot.DynamicCommands...)
	}
	return cloned
}

func cloneCommands(commands []Command) []Command {
	if len(commands) == 0 {
		return nil
	}
	cloned := make([]Command, 0, len(commands))
	for _, cmd := range commands {
		copied := cmd
		copied.Aliases = append([]string(nil), cmd.Aliases...)
		cloned = append(cloned, copied)
	}
	return cloned
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneSlice(values []any) []any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]any, len(values))
	for i, value := range values {
		cloned[i] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}

func defaultDisplayState(snapshot Snapshot) string {
	if snapshot.RegistrationState == stateRemoved {
		return displayRemoved
	}
	if !snapshot.Valid || snapshot.RegistrationState != stateInstalled {
		return projectDisplayState(snapshot)
	}

	switch snapshot.RuntimeState {
	case "starting":
		return displayEnabling
	case "running":
		return displayRunning
	case "stopping":
		if snapshot.DesiredState == "disabled" {
			return displayDisabling
		}
		return displayStopping
	case "crashed":
		return displayCrashed
	case "backoff":
		return displayBackoff
	case "dead_letter":
		return displayDeadLetter
	}

	if snapshot.DesiredState == "enabled" {
		return displayEnabled
	}
	return displayDisabled
}

func pluginStateChanged(current Snapshot, next Snapshot) bool {
	return current.RegistrationState != next.RegistrationState ||
		current.DesiredState != next.DesiredState ||
		current.RuntimeState != next.RuntimeState ||
		current.DisplayState != next.DisplayState ||
		!commandsEqual(current.Commands, next.Commands)
}

func commandsEqual(left []Command, right []Command) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name ||
			left[index].Description != right[index].Description ||
			left[index].Usage != right[index].Usage ||
			left[index].Permission != right[index].Permission ||
			left[index].CommandSource != right[index].CommandSource ||
			left[index].DeclarationID != right[index].DeclarationID ||
			!stringSlicesEqual(left[index].Aliases, right[index].Aliases) {
			return false
		}
	}
	return true
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
