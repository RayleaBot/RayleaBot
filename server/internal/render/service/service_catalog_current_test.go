package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogRemovesUnavailableSystemCache(t *testing.T) {
	root := t.TempDir()
	templates := filepath.Join(root, "templates")
	writeRenderTemplateSeed(t, templates, "help.menu")
	writeRenderTemplateSeed(t, templates, "custom.card")
	dbPath := filepath.Join(root, "state.db")
	output := filepath.Join(root, "output")
	service, closeService := openPersistentRenderService(t, root, dbPath, output, &fakeRunner{})
	t.Cleanup(func() { closeService() })
	ctx := context.Background()
	if err := os.Remove(filepath.Join(templates, "custom.card", ManifestFilename)); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		items, err := service.ListTemplates(ctx)
		if err != nil || len(items) != 1 || items[0].ID != "help.menu" {
			t.Fatalf("current catalog = %+v, error = %v", items, err)
		}
		if _, err := service.GetTemplateDetailSnapshot(ctx, "custom.card"); err == nil {
			t.Fatal("removed template still has a preview workspace")
		} else if info, ok := AsTemplateError(err); !ok || info.Code != "platform.template_not_found" {
			t.Fatalf("unexpected removed-template error: %v", err)
		}
		if _, err := service.PreviewHTML(ctx, Request{Template: "custom.card", Data: map[string]any{"title": "old"}}); err == nil {
			t.Fatal("removed template can still be previewed")
		}
		if _, err := service.templateRepo.GetTemplateDetail(ctx, "custom.card"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("removed template cache remains: %v", err)
		}
	}
	check()
	closeService()
	service, closeService = openPersistentRenderService(t, root, dbPath, output, &fakeRunner{})
	check()
}

func TestCatalogIncludesTemplateDisplayMetadata(t *testing.T) {
	root := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "help.menu")
	manifestPath := filepath.Join(root, "templates", "help.menu", ManifestFilename)
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["name"], manifest["description"] = " 帮助菜单 ", "展示可用命令。"
	payload, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	service, closeService := openPersistentRenderService(t, root, filepath.Join(root, "state.db"), filepath.Join(root, "output"), &fakeRunner{})
	defer closeService()
	items, err := service.ListTemplates(context.Background())
	if err != nil || len(items) != 1 || items[0].Name != "帮助菜单" || items[0].Description != "展示可用命令。" {
		t.Fatalf("display metadata not preserved: %+v, %v", items, err)
	}
	detail, err := service.GetTemplateDetailSnapshot(context.Background(), "help.menu")
	if err != nil || detail.Detail.Name != items[0].Name || detail.Source.ManifestJSON["name"] != items[0].Name {
		t.Fatalf("detail metadata differs: %+v, %v", detail, err)
	}
}
