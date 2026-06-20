package renderstack

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	renderbootstrap "github.com/RayleaBot/RayleaBot/server/internal/render/bootstrap"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type Deps struct {
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

func Build(deps Deps) (State, error) {
	renderer, err := buildRenderService(deps)
	if err != nil {
		return State{}, err
	}
	if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), renderer, deps.Catalog); err != nil {
		_ = renderer.Close()
		return State{}, err
	}
	return State{Renderer: renderer}, nil
}

func buildRenderService(deps Deps) (*renderservice.Service, error) {
	renderBrowserPath := renderbootstrap.PrepareBrowserPath(context.Background(), deps.Logger, deps.Discovery.RepoRoot, deps.Config.Render.BrowserPath)
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
