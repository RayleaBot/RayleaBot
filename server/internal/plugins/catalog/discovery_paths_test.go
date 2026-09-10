package catalog

import (
	"path/filepath"
	"testing"
)

func TestDiscoveryOptionsOwnScanRoots(t *testing.T) {
	root := t.TempDir()
	defaults, err := ResolveDiscovery(DiscoveryOptions{ConfigPath: filepath.Join(root, "config", "user.yaml")})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.RepoRoot != root || len(defaults.Roots) != 1 || defaults.Roots[0].Path != filepath.Join(root, "plugins", "installed") {
		t.Fatalf("default discovery roots: %#v", defaults)
	}
	if _, err := ResolveDiscovery(DiscoveryOptions{ConfigPath: filepath.Join(root, "config", "user.yaml"), PluginRepoRoot: root}); err == nil {
		t.Fatal("incomplete discovery override was accepted")
	}
	roots := []ScanRoot{{Label: "fixture", Path: filepath.Join(root, "fixture")}}
	overridden, err := ResolveDiscovery(DiscoveryOptions{PluginRepoRoot: root, PluginSchemaPath: "fixture-schema", PluginRoots: roots})
	if err != nil {
		t.Fatal(err)
	}
	roots[0].Path = "changed"
	if overridden.Roots[0].Path == "changed" {
		t.Fatal("caller mutation changed discovery specification")
	}
}
