package localaction

import (
	"context"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executeRenderImage(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "render.image") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "render.image capability is not granted",
		}
	}
	if s.renderer == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "render.image service is not available",
		}
	}

	result, err := s.renderer.Render(ctx, render.Request{
		Template: action.RenderTemplate,
		Theme:    action.RenderTheme,
		Output:   action.RenderOutput,
		Data:     action.RenderData,
	})
	if err != nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "render.image failed",
			Err:     err,
		}
	}

	return map[string]any{
		"artifact_id":   result.ArtifactID,
		"image_path":    result.ImagePath,
		"mime":          result.MIME,
		"cache_key":     result.CacheKey,
		"fallback_sent": false,
	}, nil
}

func (s *Service) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if s == nil || s.dispatcher == nil || !s.dispatcher.HasDeliverablePlugin(pluginID) {
		return
	}

	result := s.dispatcher.DispatchToPlugin(ctx, pluginID, runtime.Event{
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
	if result.Outcome == dispatch.OutcomeDelivered || s.logger == nil {
		return
	}
	s.logger.Warn(
		"config.changed event was not queued for plugin runtime",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", string(result.Outcome),
		"error_code", result.ErrorCode,
	)
}

func (s *Service) DispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	s.dispatchPluginConfigChanged(ctx, pluginID)
}

func (s *Service) executeExposeWebhook(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.webhookGateway == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}
	return s.webhookGateway.Expose(ctx, pluginID, action)
}
