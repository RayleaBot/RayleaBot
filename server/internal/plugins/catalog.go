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

type Snapshot struct {
	PluginID              string
	Name                  string
	Version               string
	Runtime               string
	Entry                 string
	Type                  string
	Description           string
	ManifestPath          string
	SourceRoot            string
	SourceRoots           []string
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
}

type Catalog struct {
	mu    sync.RWMutex
	order []string
	items map[string]Snapshot
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
		order: order,
		items: items,
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
	defer c.mu.Unlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, ErrPluginNotFound
	}

	if entry.RegistrationState != "installed" {
		return Snapshot{}, ErrStateConflict
	}

	if entry.DesiredState == desired {
		return Snapshot{}, ErrStateConflict
	}

	entry.DesiredState = desired
	entry.DisplayState = defaultDisplayState(entry)
	c.items[pluginID] = entry

	return cloneSnapshot(entry), nil
}

func (c *Catalog) SetRuntimeState(pluginID string, runtimeState string) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[pluginID]
	if !ok {
		return Snapshot{}, ErrPluginNotFound
	}

	entry.RuntimeState = runtimeState
	entry.DisplayState = defaultDisplayState(entry)
	c.items[pluginID] = entry

	return cloneSnapshot(entry), nil
}

func (c *Catalog) ApplyDesiredStates(states map[string]string) {
	if len(states) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

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

		entry.DesiredState = desired
		entry.DisplayState = defaultDisplayState(entry)
		c.items[pluginID] = entry
	}
}

func (c *Catalog) Replace(entries []Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.items = items
	c.order = order
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.SourceRoots = append([]string(nil), snapshot.SourceRoots...)
	cloned.ConflictPaths = append([]string(nil), snapshot.ConflictPaths...)
	cloned.RequiredPermissions = append([]string(nil), snapshot.RequiredPermissions...)
	cloned.OptionalPermissions = append([]string(nil), snapshot.OptionalPermissions...)
	cloned.DeclaredCapabilities = append([]string(nil), snapshot.DeclaredCapabilities...)
	cloned.PythonDependencies = append([]string(nil), snapshot.PythonDependencies...)
	cloned.NodeDependencies = append([]string(nil), snapshot.NodeDependencies...)
	cloned.ScopeHTTPHosts = append([]string(nil), snapshot.ScopeHTTPHosts...)
	cloned.ScopeStorageRoots = append([]string(nil), snapshot.ScopeStorageRoots...)
	return cloned
}

func defaultDisplayState(snapshot Snapshot) string {
	if !snapshot.Valid || snapshot.RegistrationState != "installed" {
		return snapshot.DisplayState
	}

	switch snapshot.RuntimeState {
	case "starting":
		return "enabling"
	case "running":
		return "running"
	case "stopping":
		if snapshot.DesiredState == "disabled" {
			return "disabling"
		}
		return "stopping"
	case "crashed":
		return "crashed"
	case "backoff":
		return "backoff"
	case "dead_letter":
		return "dead_letter"
	}

	if snapshot.DesiredState == "enabled" {
		return "enabled"
	}
	return "disabled"
}
