package menu

import (
	"context"
	"fmt"
	"time"

	renderartifact "github.com/RayleaBot/RayleaBot/server/internal/render/artifact"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

func (s *Service) renderBuiltinMenu(ctx context.Context, payload builtinMenuRenderData) (renderartifact.Result, error) {
	if s == nil || s.renderer == nil {
		return renderartifact.Result{}, fmt.Errorf("render service is not available")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	plugin := payload.Plugin
	if plugin == nil {
		plugin = &renderservice.PluginContext{Name: "RayleaBot"}
	}
	return s.renderer.Render(renderCtx, renderservice.Request{
		Template: builtinMenuTemplateID,
		Data:     payload.Data,
		Plugin:   plugin,
	})
}
