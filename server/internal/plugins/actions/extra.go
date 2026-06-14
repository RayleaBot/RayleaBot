package actions

import (
	"context"
	"errors"
	"strings"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func (s *Service) executeRenderImage(ctx context.Context, pluginID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "render.image") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "render.image capability is not granted",
		}
	}
	if s.renderer == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "render.image service is not available",
		}
	}

	templateID, err := s.renderer.ResolvePluginTemplate(ctx, pluginID, action.RenderTemplate)
	if err != nil {
		var renderErr *RenderTemplateError
		if errors.As(err, &renderErr) && renderErr.Code == "permission.scope_violation" {
			return nil, &runtimemanager.Error{
				Code:    "permission.scope_violation",
				Message: renderErr.Message,
				Err:     err,
			}
		}
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "render.image failed",
			Err:     err,
		}
	}

	renderData := s.renderImageData(ctx, templateID, action.RenderData, parentEvent)

	result, err := s.renderer.RenderImage(ctx, RenderImageRequest{
		Template: templateID,
		Theme:    action.RenderTheme,
		Output:   action.RenderOutput,
		Data:     renderData,
		Plugin:   s.renderPluginContext(pluginID),
	})
	if err != nil {
		return nil, &runtimemanager.Error{
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

func (s *Service) renderPluginContext(pluginID string) RenderPluginContext {
	context := RenderPluginContext{
		Name: strings.TrimSpace(pluginID),
	}
	if s == nil || s.grants == nil {
		return context
	}
	for _, snapshot := range s.grants.ListPluginSnapshots() {
		if snapshot.PluginID != pluginID {
			continue
		}
		if name := strings.TrimSpace(snapshot.Name); name != "" {
			context.Name = name
		}
		context.Version = strings.TrimSpace(snapshot.Version)
		return context
	}
	return context
}

func (s *Service) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if s == nil || s.dispatcher == nil {
		return
	}

	result := s.dispatcher(ctx, pluginID)
	if result.Delivered || s.logger == nil {
		return
	}
	s.logger.Warn(
		"config.changed event was not queued for plugin runtime",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", result.Outcome,
		"error_code", result.ErrorCode,
	)
}

func (s *Service) DispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	s.dispatchPluginConfigChanged(ctx, pluginID)
}

func (s *Service) executeExposeWebhook(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	if s == nil || s.webhookGateway == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}
	return s.webhookGateway.Expose(ctx, pluginID, action)
}
