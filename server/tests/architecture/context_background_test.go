package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type contextBackgroundAllowlist struct {
	AllowedFiles map[string]contextBackgroundEntry `json:"allowed_files"`
}

type contextBackgroundEntry struct {
	Category     string `json:"category"`
	Reason       string `json:"reason"`
	Owner        string `json:"owner"`
	TargetAction string `json:"target_action"`
	RevisitAfter string `json:"revisit_after"`
}

var validContextBackgroundCategories = map[string]struct{}{
	"adapter_runtime_root":     {},
	"cli_root":                 {},
	"compatibility_wrapper":    {},
	"dependency_diagnostic":    {},
	"nil_context_fallback":     {},
	"process_root":             {},
	"recovery_bootstrap":       {},
	"runtime_spec_wrapper":     {},
	"runtime_status_wrapper":   {},
	"shutdown_timeout":         {},
	"startup_context_fallback": {},
	"storage_bootstrap":        {},
	"websocket_close_read":     {},
	"worker_root":              {},
}

func TestContextBackgroundUsesAreAllowlisted(t *testing.T) {
	serverRoot := testServerRoot(t)
	repoRoot := filepath.Dir(serverRoot)
	registry := loadContextBackgroundAllowlist(t, repoRoot)
	found := map[string]struct{}{}

	for _, root := range []string{filepath.Join(serverRoot, "internal"), filepath.Join(serverRoot, "cmd")} {
		walkGoFiles(t, root, func(path string) {
			if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) || pathWithin(path, filepath.Join(serverRoot, "internal", "testutil")) {
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", relPath(t, repoRoot, path), err)
			}
			if !strings.Contains(string(data), "context.Background()") {
				return
			}
			rel := relPath(t, repoRoot, path)
			found[rel] = struct{}{}
			if _, ok := registry.AllowedFiles[rel]; !ok {
				t.Errorf("%s uses context.Background() without docs/engineering/context-background-allowlist.json entry", rel)
			}
		})
	}

	for rel, entry := range registry.AllowedFiles {
		if _, ok := found[rel]; !ok {
			t.Errorf("context allowlist references %s, but no context.Background() was found", rel)
		}
		if _, ok := validContextBackgroundCategories[entry.Category]; !ok {
			t.Errorf("context allowlist entry %s has invalid category %q", rel, entry.Category)
		}
		if strings.TrimSpace(entry.Reason) == "" {
			t.Errorf("context allowlist entry %s is missing reason", rel)
		}
		if strings.TrimSpace(entry.Owner) == "" {
			t.Errorf("context allowlist entry %s is missing owner", rel)
		}
		if strings.TrimSpace(entry.TargetAction) == "" {
			t.Errorf("context allowlist entry %s is missing target_action", rel)
		}
		if strings.TrimSpace(entry.RevisitAfter) == "" {
			t.Errorf("context allowlist entry %s is missing revisit_after", rel)
		}
	}
}

func loadContextBackgroundAllowlist(t *testing.T, repoRoot string) contextBackgroundAllowlist {
	t.Helper()
	path := filepath.Join(repoRoot, "docs", "engineering", "context-background-allowlist.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context allowlist: %v", err)
	}
	var registry contextBackgroundAllowlist
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("decode context allowlist: %v", err)
	}
	if registry.AllowedFiles == nil {
		t.Fatalf("context allowlist missing allowed_files")
	}
	return registry
}
