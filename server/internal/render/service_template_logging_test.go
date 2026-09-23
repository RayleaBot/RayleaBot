package render

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTemplateSyncLogsOneSummaryOnlyForChangedContent(t *testing.T) {
	root := t.TempDir()
	systemRoot := filepath.Join(root, "templates")
	pluginRoot := filepath.Join(root, "plugin", "templates")
	for _, dir := range []string{systemRoot, pluginRoot} {
		writeRenderTemplateSeed(t, dir, "card")
		writeRenderTemplateSeed(t, dir, "menu")
	}
	var output bytes.Buffer
	service, err := NewService(Options{
		RepoRoot: root, OutputRoot: filepath.Join(root, "output"),
		Store: openRenderTestStore(t), Runner: &fakeRunner{},
		Logger: slog.New(slog.NewJSONHandler(&output, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	sources := []Source{
		{PluginID: "fixture", Dir: filepath.Join(pluginRoot, "card")},
		{PluginID: "fixture", Dir: filepath.Join(pluginRoot, "menu")},
	}
	ctx := context.Background()
	syncAll := func() {
		t.Helper()
		if _, err := service.ListTemplates(ctx); err != nil {
			t.Fatal(err)
		}
		if err := service.SyncPluginTemplates(ctx, sources); err != nil {
			t.Fatal(err)
		}
	}
	syncAll()
	syncAll()
	for _, dir := range []string{systemRoot, pluginRoot} {
		if err := os.WriteFile(filepath.Join(dir, "card", "styles.css"), []byte("body { margin: 1px; }"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	syncAll()
	syncAll()
	counts := map[string][]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var entry struct {
			UpdatedCount int    `json:"updated_count"`
			SourceType   string `json:"source_type"`
			TemplateID   string `json:"template_id"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.TemplateID != "" {
			t.Fatalf("per-template diagnostics leaked into INFO: %s", line)
		}
		if entry.SourceType != "" {
			counts[entry.SourceType] = append(counts[entry.SourceType], entry.UpdatedCount)
		}
	}
	want := map[string][]int{"system": {2, 1}, "plugin": {2, 1}}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("template update summaries = %v, want %v", counts, want)
	}
}
