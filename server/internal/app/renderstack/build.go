package renderstack

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Deps struct {
	Context   context.Context
	Config    config.Config
	Logger    *slog.Logger
	Discovery runtimepaths.PluginDiscoverySpec
	Store     *storage.Store
	Catalog   *plugincatalog.Catalog
	Runner    renderbrowser.Runner
}

type State struct {
	Renderer *renderservice.Service
}

type Module = State

func Build(deps Deps) (State, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}

	renderer, err := buildRenderService(deps)
	if err != nil {
		return State{}, err
	}
	if err := SyncCatalogRenderTemplates(ctx, renderer, deps.Catalog); err != nil {
		_ = renderer.Close()
		return State{}, err
	}
	return State{Renderer: renderer}, nil
}

func SyncCatalogRenderTemplates(ctx context.Context, renderer *renderservice.Service, catalog *plugincatalog.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplateDeclarations(ctx, pluginRenderTemplateDeclarations(catalog.List()))
}

func ValidatePluginRenderTemplates(snapshot plugins.Snapshot) error {
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

func buildRenderService(deps Deps) (*renderservice.Service, error) {
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

func (s *State) Close() error {
	if s == nil || s.Renderer == nil {
		return nil
	}
	err := s.Renderer.Close()
	s.Renderer = nil
	return err
}
