package integration

import (
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestExamplePluginManifestsMatchContract(t *testing.T) {
	t.Parallel()

	validator := compileSchema(t, testutil.RepoPath(t, "contracts", "plugin-info.schema.json"))
	manifestPaths := []string{
		testutil.RepoPath(t, "examples", "plugins", "example-conversation", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "echo-go", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-config-panel", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-http-storage", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-governance-control", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-plugin-list", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-render-card", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-scheduler", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "example-webhook", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "hello-go", "info.json"),
		testutil.RepoPath(t, "examples", "plugins", "notice-logger", "info.json"),
	}

	for _, manifestPath := range manifestPaths {
		manifestPath := manifestPath
		t.Run(filepath.Base(filepath.Dir(manifestPath)), func(t *testing.T) {
			t.Parallel()

			document := loadJSONDocument(t, manifestPath)
			if err := validator.Validate(document); err != nil {
				t.Fatalf("schema validation failed for %s: %v", manifestPath, err)
			}
		})
	}
}

func TestExamplePluginManifestsDeclareV4Events(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		manifestPath string
		wantEvents   []string
	}{
		{
			name:         "echo go",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "echo-go", "info.json"),
			wantEvents:   []string{"message.group", "message.private"},
		},
		{
			name:         "example HTTP storage",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "example-http-storage", "info.json"),
			wantEvents:   []string{"message.group", "message.private"},
		},
		{
			name:         "example plugin list",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "example-plugin-list", "info.json"),
			wantEvents:   []string{"message.group", "message.private"},
		},
		{
			name:         "example render card",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "example-render-card", "info.json"),
			wantEvents:   []string{"message.group", "message.private"},
		},
		{
			name:         "example webhook",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "example-webhook", "info.json"),
			wantEvents:   []string{"webhook.received"},
		},
		{
			name:         "notice logger",
			manifestPath: testutil.RepoPath(t, "examples", "plugins", "notice-logger", "info.json"),
			wantEvents:   []string{"notice.member_increase", "notice.member_decrease"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document := loadJSONDocument(t, tc.manifestPath)
			manifest, ok := document.(map[string]any)
			if !ok {
				t.Fatalf("manifest should decode to object: %T", document)
			}

			gotEvents := sortedStringList(manifest["events"])
			if !reflect.DeepEqual(gotEvents, sortedStrings(tc.wantEvents)) {
				t.Fatalf("events mismatch for %s: got %#v want %#v", tc.manifestPath, gotEvents, sortedStrings(tc.wantEvents))
			}
			for _, legacy := range []string{"capabilities", "capability_parameters", "http_hosts", "storage_roots", "render_templates", "permissions"} {
				if _, exists := manifest[legacy]; exists {
					t.Fatalf("legacy field %s leaked into %s", legacy, tc.manifestPath)
				}
			}
		})
	}
}

func compileSchema(t *testing.T, path string) *config.Validator {
	t.Helper()

	validator, err := config.Compile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}

	return validator
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	document, err := config.LoadJSONFile(path)
	if err != nil {
		t.Fatalf("load json %s: %v", path, err)
	}

	return document
}

func sortedStringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || text == "" {
			continue
		}
		values = append(values, text)
	}
	sort.Strings(values)
	return values
}

func sortedStrings(values []string) []string {
	items := append([]string(nil), values...)
	sort.Strings(items)
	return items
}
