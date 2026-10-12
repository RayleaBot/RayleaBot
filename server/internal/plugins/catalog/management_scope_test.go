package catalog

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestManagementPageScopeProjection(t *testing.T) {
	manifest := manifestDocument{ManagementUI: &manifestManagementUI{
		Entry: "ui/index.html",
		Pages: []plugins.ManagementUIPage{
			{ID: "settings", Label: "Settings"},
			{ID: "records", Label: "Records", Scope: "bot_account"},
		},
	}}
	snapshot, err := projectManifest(manifest, filepath.Join(t.TempDir(), "info.json"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ManagementUI.Pages[0].Scope != "global" || snapshot.ManagementUI.Pages[1].Scope != "bot_account" {
		t.Fatalf("scopes = %+v", snapshot.ManagementUI.Pages)
	}
}
