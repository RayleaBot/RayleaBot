package renderaction

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/renderidentity"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
	ListPluginSnapshots() []plugins.Snapshot
}

type Renderer interface {
	ResolvePluginTemplate(context.Context, string, string) (string, error)
	RenderImage(context.Context, ImageRequest) (ImageResult, error)
	TemplateAcceptsRenderIdentity(context.Context, string) bool
}

type ImageRequest struct {
	Template string
	Theme    string
	Output   string
	Data     map[string]any
	Plugin   PluginContext
}

type PluginContext struct {
	Name    string
	Version string
}

type ImageResult struct {
	ArtifactID string
	ImagePath  string
	MIME       string
	CacheKey   string
}

type TemplateError struct {
	Code    string
	Message string
	Err     error
}

func (e *TemplateError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *TemplateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Request struct {
	PluginID      string
	Action        runtimeaction.Action
	ParentEvent   runtimeprotocol.Event
	Grants        Grants
	Renderer      Renderer
	CurrentConfig func() config.Config
}

func ExecuteImage(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "render.image") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "render.image capability is not granted",
		}
	}
	if req.Renderer == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "render.image service is not available",
		}
	}

	templateID, err := req.Renderer.ResolvePluginTemplate(ctx, req.PluginID, req.Action.RenderTemplate)
	if err != nil {
		var renderErr *TemplateError
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

	renderData := imageData(ctx, req, templateID)
	result, err := req.Renderer.RenderImage(ctx, ImageRequest{
		Template: templateID,
		Theme:    req.Action.RenderTheme,
		Output:   req.Action.RenderOutput,
		Data:     renderData,
		Plugin:   pluginContext(req.PluginID, req.Grants),
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

func imageData(ctx context.Context, req Request, templateID string) map[string]any {
	if req.Renderer == nil || !req.Renderer.TemplateAcceptsRenderIdentity(ctx, templateID) {
		return req.Action.RenderData
	}

	merged := renderidentity.CloneData(req.Action.RenderData)
	identity := renderidentity.Data(currentConfig(req), req.ParentEvent)
	merged["user"] = identity.User
	merged["permission"] = identity.Permission
	if identity.Group != nil {
		merged["group"] = identity.Group
	} else {
		delete(merged, "group")
	}
	return merged
}

func currentConfig(req Request) config.Config {
	if req.CurrentConfig == nil {
		return config.Config{}
	}
	return req.CurrentConfig()
}

func pluginContext(pluginID string, grants Grants) PluginContext {
	context := PluginContext{
		Name: strings.TrimSpace(pluginID),
	}
	if grants == nil {
		return context
	}
	for _, snapshot := range grants.ListPluginSnapshots() {
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
