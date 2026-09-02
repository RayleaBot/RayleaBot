package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func renderImageRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:          "render.image",
			Permission:      "render.image",
			RequestSchema:   "plugin-protocol.action_render_image",
			ResponseSchema:  "plugin-protocol.local_action_result",
			AccessesNetwork: true,
			WritesFile:      true,
			AuditFields:     []string{"plugin_id", "template", "output"},
			ErrorCodes: commonErrorCodes(
				"platform.invalid_request",
				"platform.upstream_response_too_large",
				"platform.render_queue_full",
				"platform.render_timeout",
				"platform.render_input_too_large",
				"platform.internal_error",
			),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeRenderImage(ctx, deps, req)
			}
		},
	}
}

func executeRenderImage(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "render.image") {
		return nil, &pluginruntime.Error{Code: "plugin.permission_denied", Message: "render.image permission is not declared"}
	}
	if deps.Renderer == nil {
		return nil, &pluginruntime.Error{Code: "plugin.internal_error", Message: "render.image service is not available"}
	}

	templateID, err := deps.Renderer.ResolvePluginTemplate(ctx, req.PluginID, req.Action.RenderTemplate)
	if err != nil {
		logRenderImageFailure(deps, req, "resolve_template", req.Action.RenderTemplate, err)
		return nil, renderImageActionError(err)
	}
	resources, cleanupResources, err := prefetchRenderImageResources(ctx, deps, req)
	if err != nil {
		logRenderImageFailure(deps, req, "prefetch_resources", templateID, err)
		return nil, err
	}
	defer cleanupResources()

	result, err := deps.Renderer.RenderImage(ctx, RenderImageRequest{
		Template:  templateID,
		Theme:     req.Action.RenderTheme,
		Output:    req.Action.RenderOutput,
		Data:      renderImageData(ctx, deps, req, templateID),
		Resources: resources,
		Plugin:    renderPluginContext(req.PluginID, deps.Permissions),
	})
	if err != nil {
		logRenderImageFailure(deps, req, "render", templateID, err)
		return nil, renderImageActionError(err)
	}
	return map[string]any{
		"artifact_id":   result.ArtifactID,
		"image_path":    result.ImagePath,
		"mime":          result.MIME,
		"cache_key":     result.CacheKey,
		"fallback_sent": false,
	}, nil
}

func renderImageActionError(err error) *pluginruntime.Error {
	var renderErr *RenderTemplateError
	if !errors.As(err, &renderErr) {
		return &pluginruntime.Error{Code: "plugin.internal_error", Message: "render.image failed", Err: err}
	}

	code := renderErr.Code
	switch code {
	case "plugin.permission_denied",
		"platform.render_queue_full",
		"platform.render_timeout",
		"platform.render_input_too_large",
		"platform.internal_error":
	default:
		code = "plugin.internal_error"
	}
	message := strings.TrimSpace(renderErr.Message)
	if message == "" {
		message = "render.image failed"
	}
	return &pluginruntime.Error{Code: code, Message: message, Err: err}
}

func logRenderImageFailure(deps Deps, req ActionRequest, phase, template string, err error) {
	if deps.Logger == nil || err == nil {
		return
	}
	message := err.Error()
	cause := deepestRenderError(err).Error()
	if deps.RedactText != nil {
		message = deps.RedactText(message)
		cause = deps.RedactText(cause)
	}
	attrs := []any{
		"component", "render",
		"plugin_id", req.PluginID,
		"request_id", req.RequestID,
		"phase", phase,
		"template", strings.TrimSpace(template),
		"error", message,
	}
	if cause != "" && cause != message {
		attrs = append(attrs, "cause", cause)
	}
	var renderErr *RenderTemplateError
	if errors.As(err, &renderErr) && strings.TrimSpace(renderErr.Code) != "" {
		attrs = append(attrs, "error_code", renderErr.Code)
	}
	displayTemplate := strings.TrimSpace(template)
	if displayTemplate == "" {
		displayTemplate = "未命名模板"
	}
	deps.Logger.Warn(fmt.Sprintf("插件 %s 的图片渲染在 %s 阶段失败，模板 %s 未生成图片。原因：%s", req.PluginID, phase, displayTemplate, cause), attrs...)
}

func deepestRenderError(err error) error {
	current := err
	for range 16 {
		next := errors.Unwrap(current)
		if next == nil {
			return current
		}
		current = next
	}
	return current
}

func renderImageData(ctx context.Context, deps Deps, req ActionRequest, templateID string) map[string]any {
	if deps.Renderer == nil || !deps.Renderer.TemplateAcceptsRenderIdentity(ctx, templateID) {
		return req.Action.RenderData
	}
	merged := CloneRenderData(req.Action.RenderData)
	identity := RenderIdentityData(currentConfig(deps), req.ParentEvent)
	merged["user"] = identity.User
	merged["permission"] = identity.Permission
	if identity.Group != nil {
		merged["group"] = identity.Group
	} else {
		delete(merged, "group")
	}
	return merged
}

func currentConfig(deps Deps) config.Config {
	if deps.CurrentConfig == nil {
		return config.Config{}
	}
	return deps.CurrentConfig()
}

func renderPluginContext(pluginID string, permissions interface {
	ListPluginSnapshots() []plugins.Snapshot
}) RenderPluginContext {
	context := RenderPluginContext{Name: strings.TrimSpace(pluginID)}
	if permissions == nil {
		return context
	}
	for _, snapshot := range permissions.ListPluginSnapshots() {
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
