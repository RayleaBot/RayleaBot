package actions

import (
	"context"

	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type renderer struct {
	service *renderservice.Service
}

func RendererFromService(service *renderservice.Service) Renderer {
	if service == nil {
		return nil
	}
	return renderer{service: service}
}

func (r renderer) ResolvePluginTemplate(ctx context.Context, pluginID, templatePath string) (string, error) {
	templateID, err := r.service.ResolvePluginTemplate(ctx, pluginID, templatePath)
	if err == nil {
		return templateID, nil
	}
	if renderErr, ok := renderservice.AsTemplateError(err); ok {
		return "", &RenderTemplateError{
			Code:    renderErr.Code,
			Message: renderErr.Message,
			Err:     err,
		}
	}
	return "", err
}

func (r renderer) RenderImage(ctx context.Context, req RenderImageRequest) (RenderImageResult, error) {
	result, err := r.service.Render(ctx, renderservice.Request{
		Template:  req.Template,
		Theme:     req.Theme,
		Output:    req.Output,
		Data:      req.Data,
		Resources: renderServiceResources(req.Resources),
		Plugin: &renderservice.PluginContext{
			Name:    req.Plugin.Name,
			Version: req.Plugin.Version,
		},
	})
	if err != nil {
		if renderErr, ok := renderservice.AsTemplateError(err); ok {
			return RenderImageResult{}, &RenderTemplateError{
				Code:    renderErr.Code,
				Message: renderErr.Message,
				Err:     err,
			}
		}
		return RenderImageResult{}, err
	}
	return RenderImageResult{
		ArtifactID: result.ArtifactID,
		ImagePath:  result.ImagePath,
		MIME:       result.MIME,
		CacheKey:   result.CacheKey,
	}, nil
}

func renderServiceResources(resources []RenderImageResource) []renderservice.RenderResource {
	result := make([]renderservice.RenderResource, 0, len(resources))
	for _, resource := range resources {
		result = append(result, renderservice.RenderResource{
			ID:     resource.ID,
			Path:   resource.Path,
			MIME:   resource.MIME,
			SHA256: resource.SHA256,
			Size:   resource.Size,
		})
	}
	return result
}

func (r renderer) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	return r.service.TemplateAcceptsRenderIdentity(ctx, templateID)
}
