package menu

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func (s *Service) renderBuiltinMenu(ctx context.Context, payload builtinMenuRenderData) (render.Result, error) {
	if s == nil || s.renderer == nil {
		return render.Result{}, fmt.Errorf("render service is not available")
	}
	renderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	plugin := payload.Plugin
	if plugin == nil {
		plugin = &render.PluginContext{Name: "RayleaBot"}
	}
	return s.renderer.Render(renderCtx, render.Request{
		Template: builtinMenuTemplateID,
		Data:     payload.Data,
		Plugin:   plugin,
	})
}
