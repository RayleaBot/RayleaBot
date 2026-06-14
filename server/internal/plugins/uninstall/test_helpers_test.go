package uninstall

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type testCatalog struct {
	order []string
	items map[string]plugins.Snapshot
}

func newTestCatalog(entries []plugins.Snapshot) *testCatalog {
	catalog := &testCatalog{}
	catalog.Replace(entries)
	return catalog
}

func (c *testCatalog) List() []plugins.Snapshot {
	result := make([]plugins.Snapshot, 0, len(c.order))
	for _, pluginID := range c.order {
		result = append(result, plugins.CloneSnapshot(c.items[pluginID]))
	}
	return result
}

func (c *testCatalog) Get(pluginID string) (plugins.Snapshot, bool) {
	snapshot, ok := c.items[pluginID]
	if !ok {
		return plugins.Snapshot{}, false
	}
	return plugins.CloneSnapshot(snapshot), true
}

func (c *testCatalog) Replace(entries []plugins.Snapshot) {
	items := make(map[string]plugins.Snapshot, len(entries))
	order := make([]string, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		items[entry.PluginID] = plugins.CloneSnapshot(entry)
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

type stubInstallRepository struct {
	saved          map[string]string
	deletedPackage string
}

func (r *stubInstallRepository) LoadDesiredStates(context.Context) (map[string]string, error) {
	if r == nil {
		return nil, nil
	}
	return r.saved, nil
}

func (r *stubInstallRepository) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	if r.saved == nil {
		r.saved = make(map[string]string)
	}
	r.saved[pluginID] = desiredState
	return nil
}

func (r *stubInstallRepository) DeleteDesiredState(_ context.Context, pluginID string) error {
	delete(r.saved, pluginID)
	return nil
}

func (r *stubInstallRepository) SavePackageMetadata(context.Context, plugins.PackageMetadata) error {
	return nil
}

func (r *stubInstallRepository) DeletePackageMetadata(_ context.Context, pluginID string) error {
	r.deletedPackage = pluginID
	return nil
}

func writeInstallSourcePlugin(t *testing.T, root, pluginID, runtimeName, entry string) string {
	t.Helper()

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create plugin root: %v", err)
	}

	manifest := map[string]any{
		"id":                      pluginID,
		"name":                    pluginID,
		"version":                 "0.1.0",
		"manifest_version":        "1",
		"plugin_protocol_version": "1",
		"type":                    "managed_runtime",
		"runtime":                 runtimeName,
		"entry":                   entry,
		"license":                 "MIT",
		"description":             "test plugin",
		"author":                  "raylea",
		"permissions": map[string]any{
			"required": []string{},
			"optional": []string{},
		},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, entry), []byte("console.log('ok')\n"), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	return root
}

func waitForTaskCompletion(t *testing.T, registry *tasks.Registry, taskID string) tasks.Snapshot {
	t.Helper()

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for task %s", taskID)
		case <-ticker.C:
			snapshot, ok := registry.Get(taskID)
			if !ok {
				t.Fatalf("task %s missing", taskID)
			}
			if snapshot.Status == tasks.StatusSucceeded || snapshot.Status == tasks.StatusFailed || snapshot.Status == tasks.StatusCancelled {
				return snapshot
			}
		}
	}
}
