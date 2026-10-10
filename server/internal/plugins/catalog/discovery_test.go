package catalog

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestTargetDiscoveryFiltersByManifestIDAndRetainsConflicts(t *testing.T) {
	repoRoot := t.TempDir()
	installed := filepath.Join(repoRoot, "plugins", "installed")
	testutil.WriteGoPluginArtifact(t, filepath.Join(installed, "different-folder-name"), "target", "0.4.0")
	peer := testutil.WriteGoPluginArtifact(t, filepath.Join(installed, "peer"), "peer", "0.4.0")
	if err := os.Remove(filepath.Join(peer, "artifact.json")); err != nil {
		t.Fatal(err)
	}
	options := DiscoverOptions{Validator: compilePluginInfoValidator(t), Roots: []ScanRoot{{Label: "plugins/installed", Path: installed}}, RepoRoot: repoRoot, PluginID: "target"}
	entries, _, err := Discover(options)
	if err != nil || len(entries) != 1 || entries[0].PluginID != "target" || !entries[0].Valid {
		t.Fatalf("target discovery: %#v %v", entries, err)
	}
	testutil.WriteGoPluginArtifact(t, filepath.Join(installed, "duplicate"), "target", "0.4.0")
	entries, _, err = Discover(options)
	if err != nil || len(entries) != 1 || entries[0].DisplayState != plugins.DisplayStateConflict {
		t.Fatalf("target conflict lost: %#v %v", entries, err)
	}
}

func TestDiscoverSkipsInternalRuntimeDirectoriesWithoutWarnings(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(filepath.Join(installedRoot, "__pycache__"), 0o755); err != nil {
		t.Fatalf("create pycache root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installedRoot, ".plugin-install-fixture"), 0o755); err != nil {
		t.Fatalf("create install staging root: %v", err)
	}

	validator := compilePluginInfoValidator(t)
	logger, stream := newPluginsTestLogger()

	snapshots, summary, err := Discover(DiscoverOptions{
		Validator: validator,
		Roots: []ScanRoot{
			{Label: "plugins/installed", Path: installedRoot},
		},
		RepoRoot: repoRoot,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("unexpected snapshots: %#v", snapshots)
	}
	if summary.SkippedCount != 0 {
		t.Fatalf("unexpected skipped count: %#v", summary)
	}
	for _, item := range stream.Snapshot() {
		t.Fatalf("unchanged internal directories should not produce default-level scan logs: %#v", item)
	}
}

func TestDiscoverWarnsForRealPluginDirectoryWithoutManifest(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(filepath.Join(installedRoot, "sample"), 0o755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}

	validator := compilePluginInfoValidator(t)
	logger, stream := newPluginsTestLogger()

	snapshots, summary, err := Discover(DiscoverOptions{
		Validator: validator,
		Roots: []ScanRoot{
			{Label: "plugins/installed", Path: installedRoot},
		},
		RepoRoot: repoRoot,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("unexpected snapshots: %#v", snapshots)
	}
	if summary.SkippedCount != 1 {
		t.Fatalf("unexpected skipped count: %#v", summary)
	}
	for _, item := range stream.Snapshot() {
		if item.Level == "warn" && item.Details["plugin_dir"] != nil {
			return
		}
	}
	t.Fatal("expected missing-info warning for real plugin directory")
}

func compilePluginInfoValidator(t *testing.T) *config.Validator {
	t.Helper()

	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}
	return validator
}

func newPluginsTestLogger() (*slog.Logger, *logging.Stream) {
	stream := logging.NewStream(16)
	writer := logging.NewSummaryWriter(io.Discard, stream, nil)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "msg"
			}
			return attr
		},
	}))
	return logger, stream
}
