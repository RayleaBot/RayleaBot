package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func buildRenderService(state appBuildState, platform appPlatform, renderRunner renderbrowser.Runner) (*renderservice.Service, error) {
	renderBrowserPath := prepareRenderBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.repoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           state.discoverySpec.repoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(platform.Storage.Path), "render"),
		Store:              platform.Storage,
		Runner:             renderRunner,
		WorkerCount:        state.core.Config.Render.WorkerCount,
		BrowserArgs:        state.core.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     state.core.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(state.core.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(state.core.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		FooterTemplate:     state.core.Config.Render.FooterTemplate,
		DefaultOutput:      state.core.Config.Render.DefaultOutput,
		DeviceScalePercent: state.core.Config.Render.DeviceScalePercent,
		Logger:             state.core.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}
