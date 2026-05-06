package server

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func TestBundledPluginManifestsMatchContract(t *testing.T) {
	t.Parallel()

	validator := compileSchema(t, filepath.Join("..", "contracts", "plugin-info.schema.json"))
	manifestPaths := []string{
		filepath.Join("..", "examples", "plugins", "echo-python", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-config-panel", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-permission-scope", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-plugin-list", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-render-card", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-scheduler", "info.json"),
		filepath.Join("..", "examples", "plugins", "example-webhook", "info.json"),
		filepath.Join("..", "examples", "plugins", "hello-python", "info.json"),
		filepath.Join("..", "examples", "plugins", "hello-node", "info.json"),
		filepath.Join("..", "examples", "plugins", "notice-logger", "info.json"),
		filepath.Join("..", "plugins", "builtin", "echo", "info.json"),
		filepath.Join("..", "plugins", "builtin", "fortune", "info.json"),
		filepath.Join("..", "plugins", "builtin", "help", "info.json"),
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

func TestBundledPluginManifestsDeclareExpectedRuntimeCapabilities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		manifestPath     string
		wantCapabilities []string
		wantRequired     []string
	}{
		{
			name:             "builtin echo",
			manifestPath:     filepath.Join("..", "plugins", "builtin", "echo", "info.json"),
			wantCapabilities: []string{"event.subscribe", "message.send"},
			wantRequired:     []string{"message.send"},
		},
		{
			name:             "builtin help",
			manifestPath:     filepath.Join("..", "plugins", "builtin", "help", "info.json"),
			wantCapabilities: []string{"event.subscribe", "message.send", "plugin.list", "render.image"},
			wantRequired:     []string{"message.send", "plugin.list", "render.image"},
		},
		{
			name:             "echo python",
			manifestPath:     filepath.Join("..", "examples", "plugins", "echo-python", "info.json"),
			wantCapabilities: []string{"event.subscribe", "message.send"},
			wantRequired:     []string{"message.send"},
		},
		{
			name:             "example permission scope",
			manifestPath:     filepath.Join("..", "examples", "plugins", "example-permission-scope", "info.json"),
			wantCapabilities: []string{"event.subscribe", "http.request", "logger.write", "storage.file"},
			wantRequired:     []string{"http.request", "logger.write", "storage.file"},
		},
		{
			name:             "example plugin list",
			manifestPath:     filepath.Join("..", "examples", "plugins", "example-plugin-list", "info.json"),
			wantCapabilities: []string{"event.subscribe", "message.send", "plugin.list"},
			wantRequired:     []string{"message.send", "plugin.list"},
		},
		{
			name:             "example render card",
			manifestPath:     filepath.Join("..", "examples", "plugins", "example-render-card", "info.json"),
			wantCapabilities: []string{"event.subscribe", "message.send", "render.image"},
			wantRequired:     []string{"message.send", "render.image"},
		},
		{
			name:             "example webhook",
			manifestPath:     filepath.Join("..", "examples", "plugins", "example-webhook", "info.json"),
			wantCapabilities: []string{"event.expose_webhook", "event.raw_payload", "event.subscribe", "logger.write"},
			wantRequired:     []string{"event.expose_webhook", "logger.write"},
		},
		{
			name:             "notice logger",
			manifestPath:     filepath.Join("..", "examples", "plugins", "notice-logger", "info.json"),
			wantCapabilities: []string{"event.subscribe", "logger.write", "storage.kv"},
			wantRequired:     []string{"logger.write", "storage.kv"},
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

			gotCapabilities := sortedStringList(manifest["capabilities"])
			if !reflect.DeepEqual(gotCapabilities, sortedStrings(tc.wantCapabilities)) {
				t.Fatalf("capabilities mismatch for %s: got %#v want %#v", tc.manifestPath, gotCapabilities, sortedStrings(tc.wantCapabilities))
			}

			permissions, ok := manifest["permissions"].(map[string]any)
			if !ok {
				t.Fatalf("permissions should decode to object: %#v", manifest["permissions"])
			}
			gotRequired := sortedStringList(permissions["required"])
			if !reflect.DeepEqual(gotRequired, sortedStrings(tc.wantRequired)) {
				t.Fatalf("required permissions mismatch for %s: got %#v want %#v", tc.manifestPath, gotRequired, sortedStrings(tc.wantRequired))
			}
		})
	}
}

func compileSchema(t *testing.T, path string) *schema.Validator {
	t.Helper()

	validator, err := schema.Compile(path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}

	return validator
}

func loadJSONDocument(t *testing.T, path string) any {
	t.Helper()

	document, err := schema.LoadJSONFile(path)
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
