package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
)

func TestExecuteRenderImageReturnsArtifact(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderService(t, renderRoot),
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "help-menu", "req_render_1", runtimeaction.Action{
		Kind:               "render.image",
		RenderTemplate:     "help.menu",
		RenderTheme:        "default",
		RenderOutput:       "png",
		RenderFallbackText: "帮助菜单暂时不可用。",
		RenderData: map[string]any{
			"title": "帮助菜单",
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}
	if result["mime"] != "image/png" {
		t.Fatalf("unexpected render mime: %#v", result["mime"])
	}
	imagePath, ok := result["image_path"].(string)
	if !ok || imagePath == "" {
		t.Fatalf("unexpected render image path: %#v", result["image_path"])
	}
	parsed, err := url.Parse(imagePath)
	if err != nil || parsed.Scheme != "file" {
		t.Fatalf("unexpected file url %q: %v", imagePath, err)
	}
	if _, err := filepath.Abs(filepath.FromSlash(parsed.Path)); err != nil {
		t.Fatalf("unexpected render file path: %v", err)
	}
	if cacheKey, ok := result["cache_key"].(string); !ok || cacheKey == "" {
		t.Fatalf("unexpected cache key: %#v", result["cache_key"])
	}
}

func TestExecuteRenderImageInjectsPluginFooter(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "help-menu",
		Name:              "帮助",
		Version:           "1.0.0",
		Valid:             true,
		RegistrationState: "installed",
	}})
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, filepath.Join("..", "..", ".."), renderRoot, runner),
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "help-menu", "req_render_footer", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "help.menu",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title":         "帮助菜单",
			"render_footer": "plugin supplied",
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}
	html := runner.lastHTML()
	if !strings.Contains(html, "Created By RayleaBot 开发版本 &amp; Plugin 帮助 1.0.0") {
		t.Fatalf("plugin footer was not injected: %s", html)
	}
	if strings.Contains(html, "plugin supplied") {
		t.Fatalf("plugin render_footer should be overwritten: %s", html)
	}
}

func TestExecuteRenderImageResolvesOwnPluginTemplateShortID(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	renderRoot := filepath.Join(t.TempDir(), "render")
	writePluginRenderTemplate(t, repoRoot, "weather-card", "card")
	renderer := newRenderServiceForRepo(t, repoRoot, renderRoot, staticRenderRunner{})
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
		PackageRootPath:   filepath.Join(repoRoot, "plugins", "installed", "weather-card"),
		RenderTemplates:   []plugins.RenderTemplate{{Path: "templates/card"}},
	}})
	if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), renderer, catalog); err != nil {
		t.Fatalf("sync plugin render templates: %v", err)
	}

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = catalog
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		renderer,
		nil,
		nil,
		nil,
	)

	result, err := application.executeLocalAction(context.Background(), "weather-card", "req_render_plugin_short", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "card",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "天气卡片",
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}
	if result["mime"] != "image/png" {
		t.Fatalf("unexpected render mime: %#v", result["mime"])
	}

	items, err := renderer.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	var found bool
	for _, item := range items {
		if item.ID == "plugin.weather-card.card" && item.Source.Type == "plugin" && item.Source.PluginID == "weather-card" && item.Source.LocalID == "card" {
			found = true
		}
	}
	if !found {
		t.Fatalf("plugin template source not listed: %#v", items)
	}
}

func TestExecuteRenderImageRejectsOtherPluginTemplate(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	renderRoot := filepath.Join(t.TempDir(), "render")
	writePluginRenderTemplate(t, repoRoot, "weather-card", "card")
	renderer := newRenderServiceForRepo(t, repoRoot, renderRoot, staticRenderRunner{})
	catalog := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather-card",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		DisplayState:      "running",
		PackageRootPath:   filepath.Join(repoRoot, "plugins", "installed", "weather-card"),
		RenderTemplates:   []plugins.RenderTemplate{{Path: "templates/card"}},
	}})
	if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), renderer, catalog); err != nil {
		t.Fatalf("sync plugin render templates: %v", err)
	}

	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.pluginStack.Plugins = catalog
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		renderer,
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalAction(context.Background(), "other-plugin", "req_render_other_plugin", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "plugin.weather-card.card",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "天气卡片",
		},
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteRenderImageRejectsUnknownOtherPluginTemplate(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, staticRenderRunner{}),
		nil,
		nil,
		nil,
	)

	_, err = application.executeLocalAction(context.Background(), "other-plugin", "req_render_unknown_other_plugin", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "plugin.weather-card.card",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "天气卡片",
		},
	})
	assertRuntimeErrorCode(t, err, "permission.scope_violation")
}

func TestExecuteRenderImageInjectsGroupIdentityFromParentEvent(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"30001"},
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, runner),
		nil,
		nil,
		nil,
	)

	_, err = application.executeLocalActionForEvent(context.Background(), "help-menu", "req_render_identity_group", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "help.menu",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "帮助菜单",
			"user": map[string]any{
				"nickname": "插件昵称",
				"id":       "plugin-user",
			},
			"group": map[string]any{
				"name": "插件群",
			},
			"permission": map[string]any{
				"level": "member",
			},
		},
	}, runtimeprotocol.Event{
		EventID:        "event-render-group",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30001",
			Nickname: "角色昵称",
			Role:     "owner",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "group",
			ID:   "2001",
			Name: "长名称测试群组",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30001",
				"sender": map[string]any{
					"user_id":  "30001",
					"nickname": "普通昵称",
					"card":     "群名片",
					"role":     "owner",
					"title":    "专属头衔",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}

	html := runner.lastHTML()
	for _, want := range []string{"群名片", "专属头衔", `<span class="identity-title"`, "ID 30001", "长名称测试群组", "超级管理员", `<span class="permission-badge`, "nk=30001"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered html missing %q:\n%s", want, html)
		}
	}
	for _, unwanted := range []string{"插件昵称", "plugin-user", "插件群"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("rendered html contains plugin identity field %q:\n%s", unwanted, html)
		}
	}
}

func TestExecuteRenderImageInjectsPrivateIdentityWithoutGroup(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, runner),
		nil,
		nil,
		nil,
	)

	_, err = application.executeLocalActionForEvent(context.Background(), "help-menu", "req_render_identity_private", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "help.menu",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "帮助菜单",
			"group": map[string]any{
				"name": "插件群",
			},
		},
	}, runtimeprotocol.Event{
		EventID:        "event-render-private",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.private",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30002",
			Nickname: "好友昵称",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "private",
			ID:   "30002",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30002",
				"sender": map[string]any{
					"user_id":  "30002",
					"nickname": "普通昵称",
					"card":     "私聊群名片",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}

	html := runner.lastHTML()
	for _, want := range []string{"好友昵称", "ID 30002", "nk=30002"} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered html missing %q:\n%s", want, html)
		}
	}
	for _, unwanted := range []string{"插件群", "私聊群名片", "群员", `<span class="permission-badge`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("private rendered html contains group-only field %q:\n%s", unwanted, html)
		}
	}
}

func TestExecuteRenderImageKeepsPrivateSuperAdminBadge(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"30002"},
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, runner),
		nil,
		nil,
		nil,
	)

	_, err = application.executeLocalActionForEvent(context.Background(), "help-menu", "req_render_identity_private_super", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "help.menu",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "帮助菜单",
		},
	}, runtimeprotocol.Event{
		EventID:        "event-render-private-super",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.private",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30002",
			Nickname: "超级用户",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "private",
			ID:   "30002",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30002",
			},
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}

	html := runner.lastHTML()
	if !strings.Contains(html, "超级管理员") || !strings.Contains(html, `<span class="permission-badge`) {
		t.Fatalf("private super admin rendered html missing badge:\n%s", html)
	}
	if strings.Contains(html, "群员") {
		t.Fatalf("private super admin rendered html should not contain member badge:\n%s", html)
	}
}

func TestExecuteRenderImageAppliesIdentityBadgeRulesToStatusPanel(t *testing.T) {
	t.Parallel()

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"30005"},
		},
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, runner),
		nil,
		nil,
		nil,
	)

	renderStatus := func(requestID string, event runtimeprotocol.Event) string {
		t.Helper()
		_, err := application.executeLocalActionForEvent(context.Background(), "status-panel", requestID, runtimeaction.Action{
			Kind:           "render.image",
			RenderTemplate: "status.panel",
			RenderTheme:    "default",
			RenderOutput:   "png",
			RenderData: map[string]any{
				"title":   "Runtime Status " + requestID,
				"status":  "ready",
				"summary": "核心服务已就绪。",
			},
		}, event)
		if err != nil {
			t.Fatalf("render.image failed: %v", err)
		}
		return runner.lastHTML()
	}

	privateHTML := renderStatus("req_render_status_private", runtimeprotocol.Event{
		EventID:        "event-render-status-private",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.private",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30004",
			Nickname: "普通好友",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "private",
			ID:   "30004",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30004",
			},
		},
	})
	if strings.Contains(privateHTML, "群员") || strings.Contains(privateHTML, `<span class="permission-badge`) {
		t.Fatalf("status private rendered html should not contain member badge:\n%s", privateHTML)
	}

	superHTML := renderStatus("req_render_status_private_super", runtimeprotocol.Event{
		EventID:        "event-render-status-private-super",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.private",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30005",
			Nickname: "超级用户",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "private",
			ID:   "30005",
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30005",
			},
		},
	})
	if !strings.Contains(superHTML, "超级管理员") || !strings.Contains(superHTML, `<span class="permission-badge`) {
		t.Fatalf("status private super admin rendered html missing badge:\n%s", superHTML)
	}

	longGroupName := "长名称测试群组"
	groupHTML := renderStatus("req_render_status_group", runtimeprotocol.Event{
		EventID:        "event-render-status-group",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30006",
			Nickname: "群名片",
			Role:     "admin",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "group",
			ID:   "2006",
			Name: longGroupName,
		},
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "30006",
				"sender": map[string]any{
					"user_id": "30006",
					"card":    "群名片",
					"role":    "admin",
					"title":   "梦忆楼",
				},
			},
		},
	})
	for _, want := range []string{longGroupName, "管理员", "梦忆楼", `<span class="identity-card__title-badge"`} {
		if !strings.Contains(groupHTML, want) {
			t.Fatalf("status group rendered html missing %q:\n%s", want, groupHTML)
		}
	}
}

func TestExecuteRenderImageLeavesNonIdentityTemplateDataUnchanged(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	writePlainRenderTemplate(t, repoRoot)

	renderRoot := filepath.Join(t.TempDir(), "render")
	runner := &captureRenderRunner{}
	application := newTestAppState(config.Config{
		Permission: config.PermissionConfig{
			AutoGrantCapabilities: []string{"render.image"},
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubLifecycleGrantRepository{grants: map[string][]plugins.PluginGrant{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		newRenderServiceForRepo(t, repoRoot, renderRoot, runner),
		nil,
		nil,
		nil,
	)

	_, err := application.executeLocalActionForEvent(context.Background(), "plain-card", "req_render_plain", runtimeaction.Action{
		Kind:           "render.image",
		RenderTemplate: "plain.card",
		RenderTheme:    "default",
		RenderOutput:   "png",
		RenderData: map[string]any{
			"title": "Plain",
			"user": map[string]any{
				"nickname": "插件昵称",
			},
			"group": map[string]any{
				"name": "插件群",
			},
			"permission": map[string]any{
				"level": "admin",
			},
		},
	}, runtimeprotocol.Event{
		EventID:        "event-render-plain",
		SourceProtocol: "onebot11",
		SourceAdapter:  "test",
		EventType:      "message.group",
		Timestamp:      time.Now().Unix(),
		Actor: &runtimeprotocol.EventActor{
			ID:       "30003",
			Nickname: "外部昵称",
			Role:     "owner",
		},
		Target: &runtimeprotocol.EventTarget{
			Type: "group",
			ID:   "2003",
			Name: "外部群组",
		},
	})
	if err != nil {
		t.Fatalf("render.image failed: %v", err)
	}

	html := runner.lastHTML()
	for _, want := range []string{"插件昵称", "插件群", "admin"} {
		if !strings.Contains(html, want) {
			t.Fatalf("plain template html missing plugin field %q:\n%s", want, html)
		}
	}
	for _, unwanted := range []string{"外部昵称", "外部群组", "owner"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("plain template html contains injected identity field %q:\n%s", unwanted, html)
		}
	}
}

func writePlainRenderTemplate(t *testing.T, repoRoot string) {
	t.Helper()

	templateRoot := filepath.Join(repoRoot, "templates", "plain.card")
	if err := os.MkdirAll(templateRoot, 0o755); err != nil {
		t.Fatalf("mkdir plain template: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "plain.card",
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 480,
  "height": 240
}`,
		"template.html": `<!doctype html>
<html lang="zh-CN">
  <head><meta charset="utf-8" /><style>{{ .Stylesheet }}</style></head>
  <body>
    <h1>{{ .title }}</h1>
    {{ with .user }}<p class="user">{{ .nickname }}</p>{{ end }}
    {{ with .group }}<p class="group">{{ .name }}</p>{{ end }}
    {{ with .permission }}<p class="permission">{{ .level }}</p>{{ end }}
  </body>
</html>`,
		"styles.css": `body { font-family: sans-serif; }`,
		"input.schema.json": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["title"],
  "properties": {
    "title": { "type": "string" }
  },
  "additionalProperties": true
}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateRoot, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write plain template %s: %v", name, err)
		}
	}
}

func writePluginRenderTemplate(t *testing.T, repoRoot, pluginID, templateID string) {
	t.Helper()

	templateDir := filepath.Join(repoRoot, "plugins", "installed", pluginID, "templates", templateID)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create plugin template dir: %v", err)
	}
	files := map[string]string{
		"template.json": `{
  "id": "` + templateID + `",
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`,
		"template.html":     "<html><body>{{ .title }}</body></html>",
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object","additionalProperties":true}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write plugin template file %s: %v", name, err)
		}
	}
}
