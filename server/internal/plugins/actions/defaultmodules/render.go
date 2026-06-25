package defaultmodules

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderidentity"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func init() {
	register(Metadata{
		Action:         "render.image",
		Capability:     "render.image",
		RequestSchema:  "plugin-protocol.action_render_image",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "template", "output"},
		ErrorCodes:     commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeRenderImage(ctx, deps, req)
		}
	})
}

func executeRenderImage(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "render.image") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "render.image capability is not declared"}
	}
	if deps.Renderer == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "render.image service is not available"}
	}

	templateID, err := deps.Renderer.ResolvePluginTemplate(ctx, req.PluginID, req.Action.RenderTemplate)
	if err != nil {
		var renderErr *actions.RenderTemplateError
		if errors.As(err, &renderErr) && renderErr.Code == "plugin.capability_violation" {
			return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: renderErr.Message, Err: err}
		}
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "render.image failed", Err: err}
	}

	result, err := deps.Renderer.RenderImage(ctx, actions.RenderImageRequest{
		Template: templateID,
		Theme:    req.Action.RenderTheme,
		Output:   req.Action.RenderOutput,
		Data:     renderImageData(ctx, deps, req, templateID),
		Plugin:   renderPluginContext(req.PluginID, deps.Capabilities),
	})
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "render.image failed", Err: err}
	}
	return map[string]any{
		"artifact_id":   result.ArtifactID,
		"image_path":    result.ImagePath,
		"mime":          result.MIME,
		"cache_key":     result.CacheKey,
		"fallback_sent": false,
	}, nil
}

func renderImageData(ctx context.Context, deps actions.Deps, req actions.ActionRequest, templateID string) map[string]any {
	if deps.Renderer == nil || !deps.Renderer.TemplateAcceptsRenderIdentity(ctx, templateID) {
		return req.Action.RenderData
	}
	merged := renderidentity.CloneData(req.Action.RenderData)
	identity := renderidentity.Data(currentConfig(deps), req.ParentEvent)
	merged["user"] = identity.User
	merged["permission"] = identity.Permission
	if identity.Group != nil {
		merged["group"] = identity.Group
	} else {
		delete(merged, "group")
	}
	return merged
}

func currentConfig(deps actions.Deps) config.Config {
	if deps.CurrentConfig == nil {
		return config.Config{}
	}
	return deps.CurrentConfig()
}

func renderPluginContext(pluginID string, capabilities interface {
	ListPluginSnapshots() []plugins.Snapshot
}) actions.RenderPluginContext {
	context := actions.RenderPluginContext{Name: strings.TrimSpace(pluginID)}
	if capabilities == nil {
		return context
	}
	for _, snapshot := range capabilities.ListPluginSnapshots() {
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
