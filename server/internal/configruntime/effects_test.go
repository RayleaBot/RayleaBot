package configruntime

import (
	"path/filepath"
	"slices"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestConfigApplyPoliciesCoverCanonicalFields(t *testing.T) {
	t.Parallel()

	cfg, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	paths := collectConfigLeafPaths(ConfigDocumentFromTyped(cfg))

	var missing []string
	for _, path := range paths {
		if _, ok := ConfigApplyPolicyForPath(path); !ok {
			missing = append(missing, path)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("missing config apply policies: %#v", missing)
	}

	pathSet := make(map[string]bool, len(paths))
	for _, path := range paths {
		pathSet[path] = true
	}
	var extra []string
	for path := range configApplyPolicies {
		if !pathSet[path] {
			extra = append(extra, path)
		}
	}
	slices.Sort(extra)
	if len(extra) != 0 {
		t.Fatalf("config apply policies for unknown fields: %#v", extra)
	}
}

func collectConfigLeafPaths(document map[string]any) []string {
	var paths []string
	collectConfigLeafPath("", document, &paths)
	slices.Sort(paths)
	return paths
}

func collectConfigLeafPath(prefix string, value any, paths *[]string) {
	if object, ok := value.(map[string]any); ok {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			collectConfigLeafPath(joinConfigPath(prefix, key), object[key], paths)
		}
		return
	}
	if prefix != "" {
		*paths = append(*paths, prefix)
	}
}
