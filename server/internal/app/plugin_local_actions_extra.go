package app

import (
	"context"
	"fmt"
	"time"

	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/runtime"
)

func (a *App) executeRenderImage(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "render.image") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "render.image capability is not granted",
		}
	}
	if a == nil || a.renderer == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "render.image service is not available",
		}
	}

	result, err := a.renderer.Render(action.RenderTemplate, action.RenderTheme, action.RenderOutput, action.RenderData)
	if err != nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "render.image failed",
			Err:     err,
		}
	}

	return map[string]any{
		"image_path":    result.ImagePath,
		"mime":          result.MIME,
		"cache_key":     result.CacheKey,
		"fallback_sent": result.FallbackSent,
	}, nil
}

func (a *App) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if a == nil || a.Dispatcher == nil || !a.Dispatcher.HasPlugin(pluginID) {
		return
	}

	result := a.Dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
		EventID:        fmt.Sprintf("config-changed-%s-%d", pluginID, time.Now().UnixNano()),
		SourceProtocol: "platform",
		SourceAdapter:  "config.internal",
		EventType:      "config.changed",
		Timestamp:      time.Now().Unix(),
		Target: &runtime.EventTarget{
			Type: "plugin",
			ID:   pluginID,
			Name: pluginID,
		},
	})
	if result.Outcome == dispatch.OutcomeDelivered || a.Logger == nil {
		return
	}
	a.Logger.Warn(
		"config.changed event was not queued for plugin runtime",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}
