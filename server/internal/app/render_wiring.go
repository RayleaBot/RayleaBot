package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type renderDeps struct {
	Context   context.Context
	Config    config.Config
	Logger    *slog.Logger
	Discovery runtimepaths.PluginDiscoverySpec
	Store     *storage.Store
	Catalog   *plugincatalog.Catalog
	Runner    renderservice.Runner
}

type appRenderState struct {
	Renderer *renderservice.Service
}

func buildRender(deps renderDeps) (appRenderState, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return appRenderState{}, err
	}

	renderer, err := buildRenderService(deps)
	if err != nil {
		return appRenderState{}, err
	}
	if err := syncCatalogRenderTemplates(ctx, renderer, deps.Catalog); err != nil {
		_ = renderer.Close()
		return appRenderState{}, err
	}
	return appRenderState{Renderer: renderer}, nil
}

func syncCatalogRenderTemplates(ctx context.Context, renderer *renderservice.Service, catalog *plugincatalog.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplateDeclarations(ctx, pluginRenderTemplateDeclarations(catalog.List()))
}

func validatePluginRenderTemplates(snapshot plugins.Snapshot) error {
	return renderservice.ValidatePluginTemplateDeclarations(pluginRenderTemplateDeclarations([]plugins.Snapshot{snapshot}))
}

func pluginRenderTemplateDeclarations(snapshots []plugins.Snapshot) []renderservice.PluginTemplateDeclaration {
	var declarations []renderservice.PluginTemplateDeclaration
	for _, snapshot := range snapshots {
		for _, declared := range snapshot.RenderTemplates {
			declarations = append(declarations, renderservice.PluginTemplateDeclaration{
				PluginID:          snapshot.PluginID,
				Path:              declared.Path,
				PackageRootPath:   snapshot.PackageRootPath,
				Valid:             snapshot.Valid,
				RegistrationState: snapshot.RegistrationState,
			})
		}
	}
	return declarations
}

func buildRenderService(deps renderDeps) (*renderservice.Service, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	renderBrowserPath := prepareBrowserPath(ctx, deps.Logger, deps.Discovery.RepoRoot, deps.Config.Render.BrowserPath)
	renderService, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           deps.Discovery.RepoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(deps.Store.Path), "render"),
		Store:              deps.Store,
		Runner:             deps.Runner,
		WorkerCount:        deps.Config.Render.WorkerCount,
		BrowserArgs:        deps.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     deps.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(deps.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(deps.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		FooterTemplate:     deps.Config.Render.FooterTemplate,
		DefaultOutput:      deps.Config.Render.DefaultOutput,
		DeviceScalePercent: deps.Config.Render.DeviceScalePercent,
		Logger:             deps.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

var resolveManagedBrowserPath = func(ctx context.Context, repoRoot string) (string, error) {
	return deps.NewRuntime(repoRoot).ResolveEntrypoint(ctx, "chromium", "browser")
}

func prepareBrowserPath(ctx context.Context, logger *slog.Logger, repoRoot string, configuredPath string) string {
	browserPath := strings.TrimSpace(configuredPath)
	if browserPath != "" {
		return browserPath
	}

	managedBrowserPath, err := resolveManagedBrowserPath(ctx, repoRoot)
	if err != nil {
		if logger != nil {
			logger.Warn(
				"托管 Chromium 暂不可用，图片渲染等待运行环境准备",
				"component", "render",
				"code", "platform.resource_missing",
				"err", logpath.Error(repoRoot, err, repoRoot),
			)
		}
		return ""
	}

	if logger != nil {
		browserDisplayPath := logpath.Display(repoRoot, managedBrowserPath)
		logger.Info(
			"托管 Chromium 已就绪，浏览器路径："+browserDisplayPath,
			"component", "render",
			"browser_path", browserDisplayPath,
		)
	}
	return managedBrowserPath
}

func (s *appRenderState) Close() error {
	if s == nil || s.Renderer == nil {
		return nil
	}
	err := s.Renderer.Close()
	s.Renderer = nil
	return err
}
