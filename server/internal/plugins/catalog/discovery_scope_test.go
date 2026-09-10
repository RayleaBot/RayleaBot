package catalog_test

import (
	"testing"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestPluginDiscoveryContextUsesOnlyInstalledRoot(t *testing.T) {
	t.Parallel()

	_, _, roots, err := plugincatalog.DiscoveryContext(testutil.RepoPath(t, "contracts", "config.user.schema.json"))
	if err != nil {
		t.Fatalf("plugincatalog.DiscoveryContext failed: %v", err)
	}
	if len(roots) != 1 || roots[0].Label != "plugins/installed" {
		t.Fatalf("expected only installed root, got %#v", roots)
	}
}
